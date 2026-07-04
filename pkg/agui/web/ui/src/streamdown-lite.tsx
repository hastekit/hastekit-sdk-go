import React from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

// Lightweight drop-in replacement for the `streamdown` package that
// CopilotKit v2 statically imports for assistant-message rendering.
//
// The real streamdown bundles Shiki (a grammar chunk per language),
// Mermaid, and Cytoscape — together ~14MB of lazy chunks that Vite
// would emit into ../static and bake into the Go binary. vite.config.ts
// aliases `streamdown` to this module so none of that weight ever
// enters the build; the binary keeps only React + CopilotKit core.
//
// The trade-off: assistant markdown still renders (prose, lists,
// tables, links via react-markdown + GFM) and code blocks show as
// plain unhighlighted <pre>. Mermaid/diagram fences render as their
// source text instead of an SVG. For a built-in agent chat UI that's
// a fine default; rebuild from the real streamdown if you need rich
// diagrams (see ui/README.md).

export interface StreamdownProps {
  children?: React.ReactNode;
  className?: string;
  [key: string]: unknown;
}

export const Streamdown = React.memo(function Streamdown({
  children,
  className,
}: StreamdownProps) {
  const content = typeof children === "string" ? children : "";
  return (
    <div className={className} data-streamdown-lite="">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        // react-markdown strips data: URLs by default; allow them so
        // generated images (rendered as ![](data:image/...;base64,...))
        // display. Content comes from the user's own agent.
        urlTransform={(url) => url}
        components={{
          img: (props) => (
            <img {...props} style={{ maxWidth: "100%", borderRadius: 8, ...(props.style || {}) }} />
          ),
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
});

// Defensive extras so any other named import from "streamdown"
// resolves. CopilotKit's react-core/v2 only imports `Streamdown`
// today, but mirroring the surface keeps the alias robust across
// minor CopilotKit bumps.
export const Block = Streamdown;
export const StreamdownContext = React.createContext<unknown>(null);
export const defaultRehypePlugins: unknown[] = [];
export const defaultRemarkPlugins: unknown[] = [remarkGfm];
export function parseMarkdownIntoBlocks(md: string): string[] {
  return [md];
}

export default Streamdown;
