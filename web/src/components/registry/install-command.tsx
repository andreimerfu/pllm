import { useState } from "react";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import type { RegistryPackage } from "@/types/api";

// Translate a RegistryPackage into the command a developer would actually
// paste. Only two conventions matter today: `npx -y` for npm and `uvx` for
// PyPI. OCI images get `docker pull`. Everything else falls through.
export function installCommand(pkg?: RegistryPackage): string | null {
  if (!pkg || !pkg.registry_type || !pkg.identifier || !pkg.version) return null;
  const t = pkg.registry_type.toLowerCase();
  const ref = `${pkg.identifier}@${pkg.version}`;
  switch (t) {
    case "npm":
      return `npx -y ${ref}`;
    case "pypi":
      return `uvx ${pkg.identifier}==${pkg.version}`;
    case "oci":
    case "docker":
      return `docker pull ${pkg.identifier}:${pkg.version}`;
    case "go":
      return `go install ${pkg.identifier}@${pkg.version}`;
    case "cargo":
      return `cargo install ${pkg.identifier} --version ${pkg.version}`;
  }
  return null;
}

// Monospace block with a copy button. Subtle enough to drop into card
// footers; prominent enough to invite a click.
export function InstallCommand({
  packages,
  className,
}: {
  packages?: RegistryPackage[];
  className?: string;
}) {
  const [copied, setCopied] = useState(false);
  const primary = (packages ?? [])[0];
  const cmd = installCommand(primary);
  if (!cmd) return null;

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(cmd);
      setCopied(true);
      setTimeout(() => setCopied(false), 1400);
    } catch {
      /* ignore */
    }
  };

  return (
    <button
      type="button"
      onClick={copy}
      className={cn(
        "group flex items-center gap-2 w-full min-w-0 px-3 py-2 rounded-md",
        "bg-muted/50 hover:bg-muted text-left transition-colors",
        "border border-border/60 hover:border-border",
        className,
      )}
      title="Copy install command"
    >
      <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground shrink-0">
        $
      </span>
      <code className="text-xs font-mono truncate flex-1">{cmd}</code>
      <Icon
        icon={copied ? icons.check : icons.copy}
        className={cn(
          "h-3.5 w-3.5 shrink-0 transition-colors",
          copied ? "text-emerald-500" : "text-muted-foreground group-hover:text-foreground",
        )}
      />
    </button>
  );
}
