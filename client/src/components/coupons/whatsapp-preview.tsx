import { cn } from '@/lib/utils';
import { useMemo } from 'react';

interface WhatsAppPreviewProps {
  text: string;
  className?: string;
  /**
   * Optional caption shown above the bubble. Defaults to
   * "Visualização do WhatsApp". Set null to hide.
   */
  title?: string | null;
  /**
   * Optional placeholder when text is empty. Defaults to a hint.
   */
  emptyHint?: string;
}

/**
 * WhatsApp-flavored bubble preview. Renders the same `*bold*`,
 * `_italic_`, `~strike~`, and `\`mono\`` markup that WhatsApp itself
 * does, so operators see exactly how a coupon line / message lands
 * in the chat without sending a test message.
 *
 * Used in the cupom Simples form (under the Mensagem field) and on
 * the Cupons Avançados rule editor (showing the linked cupom's text).
 *
 * Output is built from a small token-walk that recognizes the four
 * delimiters and escapes the rest. Anything unknown falls through as
 * literal text — no full markdown engine, just enough to mirror
 * WhatsApp's documented formatting subset.
 */
export function WhatsAppPreview({
  text,
  className,
  title = 'Visualização do WhatsApp',
  emptyHint = 'Digite a mensagem para ver o preview...',
}: WhatsAppPreviewProps) {
  const formatted = useMemo(() => formatWhatsAppText(text), [text]);
  const time = useMemo(() => {
    const d = new Date();
    return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
  }, []);

  const isEmpty = !text || !text.trim();

  return (
    <div className={cn('flex flex-col gap-2', className)}>
      {title && <span className="text-xs text-muted-foreground">{title}</span>}
      <div
        className="rounded-2xl px-4 py-3"
        style={{ backgroundColor: 'rgba(141, 45, 83, 0.08)' }}
      >
        <div
          className="rounded-2xl bg-white px-3 py-2 ml-auto max-w-[85%] text-sm shadow-sm whitespace-pre-wrap break-words"
          style={{ color: '#1f2937' }}
        >
          {isEmpty ? (
            <span className="text-muted-foreground italic">{emptyHint}</span>
          ) : (
            <span dangerouslySetInnerHTML={{ __html: formatted }} />
          )}
          <div className="text-[10px] text-right mt-1 text-muted-foreground">{time}</div>
        </div>
      </div>
    </div>
  );
}

// formatWhatsAppText turns the WhatsApp markup subset into safe HTML.
// We escape the entire input first, then run delimiter passes on the
// escaped string — that order guarantees `<script>` from user input
// can never reach the DOM as a tag, while our wrapping <strong>/<em>
// tags inserted afterwards survive untouched.
function formatWhatsAppText(input: string): string {
  if (!input) return '';
  let out = escapeHtml(input);
  // WhatsApp delimiters are word-bounded: `*foo*` requires non-space
  // immediately inside. The regex enforces that to avoid matching
  // bullet lists like "* item". Order matters: monospace first so
  // its contents aren't reformatted, then strikethrough, italic, bold.
  out = out.replace(/`([^`\n]+?)`/g, '<code>$1</code>');
  out = out.replace(/(^|\s)~([^~\s][^~\n]*?[^~\s]|[^~\s])~(?=\s|$|[.,!?;:])/g, '$1<s>$2</s>');
  out = out.replace(/(^|\s)_([^_\s][^_\n]*?[^_\s]|[^_\s])_(?=\s|$|[.,!?;:])/g, '$1<em>$2</em>');
  out = out.replace(/(^|\s)\*([^*\s][^*\n]*?[^*\s]|[^*\s])\*(?=\s|$|[.,!?;:])/g, '$1<strong>$2</strong>');
  return out;
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}
