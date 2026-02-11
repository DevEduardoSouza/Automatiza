import type { CSSProperties, ReactNode } from 'react';

interface Props {
  text: string;
  className?: string;
}

const BOLD_STYLE: CSSProperties = { fontWeight: 700 };
const ITALIC_STYLE: CSSProperties = { fontStyle: 'italic' };
const STRIKE_STYLE: CSSProperties = { textDecoration: 'line-through' };
const CODE_STYLE: CSSProperties = {
  fontFamily: 'ui-monospace, monospace',
  fontSize: '0.85em',
  backgroundColor: 'rgba(0,0,0,0.06)',
  borderRadius: 4,
  padding: '1px 4px',
};
const QUOTE_STYLE: CSSProperties = {
  borderLeft: '3px solid #a1a1aa',
  paddingLeft: 8,
  color: '#71717a',
};

type MarkerChar = '*' | '_' | '~' | '`';

interface MarkerDef {
  style: CSSProperties;
  allowNesting: boolean;
}

const MARKERS: Record<MarkerChar, MarkerDef> = {
  '*': { style: BOLD_STYLE, allowNesting: true },
  _: { style: ITALIC_STYLE, allowNesting: true },
  '~': { style: STRIKE_STYLE, allowNesting: true },
  '`': { style: CODE_STYLE, allowNesting: false },
};

function isMarker(ch: string): ch is MarkerChar {
  return ch === '*' || ch === '_' || ch === '~' || ch === '`';
}

interface KeyCounter {
  next(): number;
}

function createKeyCounter(): KeyCounter {
  let count = 0;
  return {
    next() {
      return count++;
    },
  };
}

function parseInline(text: string, counter: KeyCounter): ReactNode[] {
  const nodes: ReactNode[] = [];
  let buf = '';
  let i = 0;

  const flush = () => {
    if (buf) {
      nodes.push(buf);
      buf = '';
    }
  };

  while (i < text.length) {
    // Triple backtick code block
    if (text[i] === '`' && text[i + 1] === '`' && text[i + 2] === '`') {
      const start = i + 3;
      const end = text.indexOf('```', start);
      if (end !== -1) {
        flush();
        nodes.push(
          <span key={counter.next()} style={CODE_STYLE}>
            {text.slice(start, end)}
          </span>
        );
        i = end + 3;
        continue;
      }
    }

    const ch = text[i];

    if (isMarker(ch)) {
      const def = MARKERS[ch];

      // WhatsApp rule: no space immediately after opening marker
      if (i + 1 < text.length && text[i + 1] !== ' ' && text[i + 1] !== ch) {
        // Search for a valid closing marker
        let j = i + 2;
        let found = false;

        while (j < text.length) {
          if (text[j] === ch && text[j - 1] !== ' ') {
            // Found valid close
            flush();
            const inner = text.slice(i + 1, j);
            const children = def.allowNesting ? parseInline(inner, counter) : inner;
            nodes.push(
              <span key={counter.next()} style={def.style}>
                {children}
              </span>
            );
            i = j + 1;
            found = true;
            break;
          }
          j++;
        }

        if (!found) {
          buf += ch;
          i++;
        }
      } else {
        buf += ch;
        i++;
      }
    } else {
      buf += ch;
      i++;
    }
  }

  flush();
  return nodes;
}

function parseLine(line: string, key: number, counter: KeyCounter): ReactNode {
  if (line.startsWith('> ')) {
    return (
      <div key={key} style={QUOTE_STYLE}>
        {parseInline(line.slice(2), counter)}
      </div>
    );
  }
  return <div key={key}>{line === '' ? '\u00A0' : parseInline(line, counter)}</div>;
}

export function WhatsAppFormattedText({ text, className }: Props) {
  const counter = createKeyCounter();
  const lines = text.split('\n');
  return <div className={className}>{lines.map((line, i) => parseLine(line, i, counter))}</div>;
}
