import { cn } from "@/lib/utils";

// Small mono chip labeling the source ecosystem of a package:
// npm / pypi / docker / oci / go / cargo / etc. Drop in anywhere we
// want a caller to recognize "this comes from npm" at a glance.
const PKG_TONES: Record<string, string> = {
  npm: "text-red-700 dark:text-red-300 bg-red-500/10 ring-red-500/20",
  pypi: "text-blue-700 dark:text-blue-300 bg-blue-500/10 ring-blue-500/20",
  oci: "text-cyan-700 dark:text-cyan-300 bg-cyan-500/10 ring-cyan-500/20",
  docker: "text-cyan-700 dark:text-cyan-300 bg-cyan-500/10 ring-cyan-500/20",
  go: "text-teal-700 dark:text-teal-300 bg-teal-500/10 ring-teal-500/20",
  cargo: "text-orange-700 dark:text-orange-300 bg-orange-500/10 ring-orange-500/20",
  maven: "text-fuchsia-700 dark:text-fuchsia-300 bg-fuchsia-500/10 ring-fuchsia-500/20",
  rubygems: "text-rose-700 dark:text-rose-300 bg-rose-500/10 ring-rose-500/20",
  nuget: "text-violet-700 dark:text-violet-300 bg-violet-500/10 ring-violet-500/20",
};

export function PackageChip({ type }: { type?: string }) {
  if (!type) return null;
  const key = type.toLowerCase();
  const tone = PKG_TONES[key] ?? "text-muted-foreground bg-muted ring-border";
  return (
    <span
      className={cn(
        "inline-flex items-center px-1.5 h-[18px] rounded text-[10px] font-mono uppercase tracking-wider ring-1",
        tone,
      )}
    >
      {key}
    </span>
  );
}
