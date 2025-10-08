interface DetailItemProps {
  label: string;
  value?: string | number | null;
  children?: React.ReactNode;
}

export const DetailItem = ({ label, value, children }: DetailItemProps) => (
  <div className="flex flex-col gap-1">
    <dt className="text-sm font-medium text-muted-foreground">{label}</dt>
    <dd className="text-sm">{children || value || 'N/A'}</dd>
  </div>
);
