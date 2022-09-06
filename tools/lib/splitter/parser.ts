'use strict';
/* Imports */
import { File, Tag, Line, LineKind } from './file';

/* Helpers */
const top = <T>(arr: T[]): T | null =>
  arr.length > 0 ? arr[arr.length - 1] : null;
const quote = (s: string): string => JSON.stringify(s);

export function skipWhitespaces(line: string, start: number): number {
  let end = start;

  loop: for (; end < line.length; end++) {
    switch (line[end]) {
      case ' ':
      case '\t':
        // Eat.
        break;

      default:
        break loop;
    }
  }

  return end;
}

export function uncommentLine(line: string): string {
  const idx = skipWhitespaces(line, 0);
  if (idx >= line.length - 1) {
    return line;
  }

  if (line[idx] !== '/' || line[idx + 1] !== '/') {
    return line;
  }

  const prefix = line.slice(0, idx);
  let suffix = line.slice(idx + 2);
  if (suffix.length > 0 && suffix[0] === ' ') {
    suffix = suffix.slice(1);
  }

  return prefix + suffix;
}

/* Parser */
export enum MagicMarker {
  Start = '>',
  End = '<',
  Line = '^'
}

export type MagicLine = {
  marker: MagicMarker;
  kind: string;
  args: string[];
  options: Record<string, string | null>;
};

export class MagicLineParser {
  private _i: number = 0;

  constructor(private _line: string = '') {}

  reset(line: string): void {
    this._i = 0;
    this._line = line;
  }

  parse(marker?: MagicMarker | null): MagicLine | null {
    if (marker == null) {
      marker = this._isMagicLine();
    }

    if (marker === null) {
      return null;
    }

    this._skipWhitespaces();

    // Parse the magic line.
    const magicLine = this._parseMagicLine(marker);

    this._skipWhitespaces();

    // Eat '\n' and '\r'.
    while (
      this._i < this._line.length &&
      (this._line[this._i] === '\r' || this._line[this._i] === '\n')
    ) {
      this._i++;
    }

    if (this._i < this._line.length) {
      throw new Error(`Expected EOL, got ${quote(this._line[this._i])}`);
    }

    return magicLine;
  }

  private _isMagicLine(): MagicMarker | null {
    this._skipWhitespaces();

    if (
      this._i >= this._line.length - 2 ||
      this._line[this._i] != '/' ||
      this._line[this._i + 1] != '/'
    ) {
      return null;
    }

    const marker = this._line[this._i + 2] as MagicLine | string;

    switch (marker) {
      case MagicMarker.Start:
      case MagicMarker.End:
      case MagicMarker.Line:
        this._i += 3;
        return marker;

      default:
        return null;
    }
  }

  private _parseMagicLine(marker: MagicMarker): MagicLine {
    // Parse the kind.
    const kind = this._parseString();

    const magicLine: MagicLine = {
      marker,
      kind,
      args: [],
      options: {}
    };

    // Parse args.
    while (true) {
      const size = this._skipWhitespaces();
      if (this._i >= this._line.length || size === 0) {
        break;
      }

      const ch = this._line[this._i];
      if (ch === ':' || ch === '\n' || ch === '\r') {
        break;
      }

      const arg = this._parseString();

      magicLine.args.push(arg);
    }

    // Parse options.
    if (this._i >= this._line.length) {
      return magicLine;
    }

    if (this._line[this._i] === ':') {
      this._i++;

      let foundFirst = false;
      while (this._i < this._line.length) {
        this._skipWhitespaces();
        if (
          this._i >= this._line.length ||
          this._line[this._i] === '\n' ||
          this._line[this._i] === '\r'
        ) {
          break;
        }

        if (foundFirst) {
          this._expect(',');

          this._skipWhitespaces();
        }

        const { key, value } = this._parseOption();

        magicLine.options[key] = value;

        foundFirst = true;
      }
    }

    return magicLine;
  }

  private _parseString(): string {
    if (this._i >= this._line.length) {
      throw new Error('Reached EOL');
    }

    const ch = this._line[this._i];
    switch (ch) {
      case '"':
      case "'":
        return this._parseQuotedString(ch);

      default:
        return this._parseUnquotedString();
    }
  }

  private _parseUnquotedString(): string {
    if (this._i >= this._line.length) {
      throw new Error('Reached EOL');
    }

    const start = this._i;
    let end = this._i;
    for (; end < this._line.length; end++) {
      const ch = this._line[end];

      const valid =
        (ch >= 'a' && ch <= 'z') ||
        (ch >= 'A' && ch <= 'Z') ||
        (ch >= '0' && ch <= '9') ||
        ch == '-' ||
        ch == '_';

      if (!valid) {
        break;
      }
    }

    if (start === end) {
      throw new Error('Empty string');
    }

    this._i = end;

    return this._line.slice(start, end);
  }

  private _parseQuotedString(q: string): string {
    if (this._i >= this._line.length) {
      throw new Error('Reached EOL');
    }

    this._expect(q);

    let escaped = false;
    let closed = false;
    let buf = '';
    let end = this._i;

    this._i--;

    loop: for (; end < this._line.length; end++) {
      const ch = this._line[end];

      switch (ch) {
        case '\n':
        case '\r':
          throw new Error('Unexpected newline');
      }

      if (!escaped) {
        switch (ch) {
          case q:
            end++;
            closed = true;
            break loop;

          case '\\':
            escaped = true;
            continue;
        }
      }

      buf += ch;
      escaped = false;
    }

    if (escaped || !closed) {
      throw new Error('Unexpected end of the string');
    }

    this._i = end;

    return buf;
  }

  private _parseOption(): { key: string; value: string | null } {
    if (this._i >= this._line.length) {
      throw new Error('Reached EOL');
    }

    // Parse the key.
    const key = this._parseUnquotedString();

    // Parse the '='.
    this._skipWhitespaces();

    if (this._i >= this._line.length || this._line[this._i] !== '=') {
      return { key, value: null };
    }

    this._i++;

    // Parse the value.
    this._skipWhitespaces();

    const value = this._parseString();

    return { key, value };
  }

  private _skipWhitespaces(): number {
    const start = this._i;
    const end = skipWhitespaces(this._line, start);

    this._i = end;

    return end - start;
  }

  private _expect(expected: string): void {
    if (this._i >= this._line.length) {
      throw new Error('Reached EOL');
    }

    const actual = this._line[this._i];
    if (actual !== expected) {
      throw new Error(`Expected ${quote(expected)}, got ${quote(actual)}`);
    }
    this._i++;
  }
}

export class FileParser {
  parse(path: string, source: string): File {
    const stack: Tag[] = [];
    const tags: Record<string, Tag> = {};
    let lines: Line[] = [];

    const magicLineParser = new MagicLineParser();

    let removeAfter = 0;
    for (let line of source.split('\n')) {
      // Skip the line.
      if (removeAfter > 0) {
        removeAfter--;
        continue;
      }

      // Parse the line.
      magicLineParser.reset(line);

      const magicLine = magicLineParser.parse();

      // Check if it is just a line.
      if (magicLine == null) {
        let tagName: string | null = null;
        const tag = top(stack);
        if (tag !== null) {
          if (tag.uncommentLines !== false) {
            line = uncommentLine(line);
          }

          tagName = tag.name;
        }

        lines.push({
          kind: LineKind.Text,
          tag: tagName,
          text: line
        });

        continue;
      }

      // Process the magic line.
      switch (magicLine.marker) {
        case MagicMarker.Start:
          switch (magicLine.kind) {
            case 'stage': {
              if (magicLine.args.length !== 1) {
                throw new Error(
                  `Wrong arity for ${quote(magicLine.kind)}: got ${
                    magicLine.args.length
                  }, want 1`
                );
              }

              const tag: Tag = {
                enabled: false,
                name: magicLine.args[0],
                replaces: null,
                uncommentLines: false
              };

              stack.push(tag);
              tags[tag.name] = tag;
              lines.push({
                kind: LineKind.TagStart,
                tag: tag.name
              });
              break;
            }

            case 'snippet': {
              const tag: Tag = {
                enabled: false,
                name: '',
                replaces: null,
                uncommentLines: false
              };

              if (magicLine.args.length === 1) {
                tag.name = magicLine.args[0];
              } else if (magicLine.args.length === 3) {
                tag.name = magicLine.args[0];

                if (magicLine.args[1] !== 'replaces') {
                  throw new Error(
                    `Unknown 2nd argument in ${quote(magicLine.kind)}: ${quote(
                      magicLine.args[1]
                    )}`
                  );
                }

                tag.replaces = magicLine.args[2];
              } else {
                throw new Error(
                  `Wrong arity for ${quote(magicLine.kind)}: got ${
                    magicLine.args.length
                  }, want 1 or 3`
                );
              }

              if (magicLine.options.hasOwnProperty('uncomment-lines')) {
                const v = magicLine.options['uncomment-lines'];
                if (v !== null && v.length > 0) {
                  tag.uncommentLines = v;
                } else {
                  tag.uncommentLines = true;
                }
              }

              stack.push(tag);
              tags[tag.name] = tag;
              lines.push({
                kind: LineKind.TagStart,
                tag: tag.name
              });
              break;
            }

            default:
              throw new Error(
                `Unknwon ${quote(magicLine.marker)} kind: ${quote(
                  magicLine.kind
                )}`
              );
          }
          break;

        case MagicMarker.End:
          switch (magicLine.kind) {
            case 'stage':
            case 'snippet': {
              if (magicLine.args.length !== 1) {
                throw new Error(
                  `Wrong arity for ${quote(magicLine.kind)}: got ${
                    magicLine.args.length
                  }, want 1`
                );
              }

              const name = magicLine.args[0];

              const topTag = stack.pop();
              if (topTag === undefined) {
                throw new Error('Tags stack is empty');
              }

              if (topTag.name !== name) {
                throw new Error(
                  `Top tag gas different name: expected ${quote(
                    name
                  )}, got ${quote(topTag.name)}`
                );
              }

              lines.push({
                kind: LineKind.TagEnd,
                tag: name
              });
              break;
            }

            default:
              throw new Error(
                `Unknwon ${quote(magicLine.marker)} kind: ${quote(
                  magicLine.kind
                )}`
              );
          }
          break;

        case MagicMarker.Line:
          switch (magicLine.kind) {
            case 'remove-lines': {
              if (magicLine.args.length !== 0) {
                throw new Error(
                  `Wrong arity for ${quote(magicLine.kind)}: got ${
                    magicLine.args.length
                  }, want 0`
                );
              }

              if (magicLine.options.hasOwnProperty('before')) {
                const v = parseInt(magicLine.options.before ?? '', 10);
                if (isNaN(v)) {
                  throw new Error(
                    `Broken value in the "before" option: ${quote(
                      magicLine.options.before ?? '<null>'
                    )}`
                  );
                }

                // TODO.
                lines = lines.slice(0, -1 * v);
              }

              if (magicLine.options.hasOwnProperty('after')) {
                const v = parseInt(magicLine.options.after ?? '', 10);
                if (isNaN(v)) {
                  throw new Error(
                    `Broken value in the "after" option: ${quote(
                      magicLine.options.after ?? '<null>'
                    )}`
                  );
                }

                removeAfter += v;
              }
              break;
            }

            default:
              throw new Error(
                `Unknwon ${quote(magicLine.marker)} kind: ${quote(
                  magicLine.kind
                )}`
              );
          }
          break;

        default:
          throw new Error(`Unknwon marker ${quote(magicLine.marker)}`);
      }
    }

    // Pop extra tags.
    while (true) {
      const topTag = stack.pop();
      if (topTag === undefined) {
        break;
      }

      lines.push({
        kind: LineKind.TagEnd,
        tag: topTag.name
      });
    }

    return new File(path, lines, tags);
  }
}
