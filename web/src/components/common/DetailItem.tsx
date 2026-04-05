interface DetailItemProps {
  label: string;
  value?: string | number | null;
  children?: React.ReactNode;
}

export const DetailItem = ({ label, value, children }: DetailItemProps) => (
  <div className="flex flex-col gap-1">
    <dt className="text-[12px] text-muted-foreground">{label}</dt>
    <dd className="text-[12px] text-foreground">{children || value || 'N/A'}</dd>
  </div>
);
