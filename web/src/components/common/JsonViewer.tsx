import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';

interface JsonViewerProps {
  title: string;
  data: object | null;
}

export const JsonViewer = ({ title, data }: JsonViewerProps) => {
  if (!data || Object.keys(data).length === 0) {
    return null;
  }

  return (
    <div>
      <h3 className="text-[12px] font-medium mb-2">{title}</h3>
      <div className="rounded-md bg-background font-mono text-[12px]">
        <SyntaxHighlighter
          language="json"
          style={vscDarkPlus}
          customStyle={{ margin: 0, borderRadius: '0.5rem', fontSize: '0.75rem' }}
        >
          {JSON.stringify(data, null, 2)}
        </SyntaxHighlighter>
      </div>
    </div>
  );
};
