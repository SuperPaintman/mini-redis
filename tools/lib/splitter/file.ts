'use strict';

export type Tag = {
  enabled: boolean;
  name: string;
  replaces: string | null;
  uncommentLines: string | boolean;
};

export enum LineKind {
  Text,
  TagStart,
  TagEnd
}

export type LineText = {
  kind: LineKind.Text;
  text: string;
  tag: string | null;
};

export type LineTagStart = {
  kind: LineKind.TagStart;
  tag: string;
};

export type LineTagEnd = {
  kind: LineKind.TagEnd;
  tag: string;
};

export type Line = LineText | LineTagStart | LineTagEnd;

export class File {
  constructor(
    private _path: string,
    private _language: string,
    private _lines: Line[],
    private _tags: Record<string, Tag>
  ) {}

  path(): string {
    return this._path;
  }

  language(): string {
    return this._language;
  }

  tags(): Record<string, Tag> {
    return this._tags;
  }

  lines(): string[] {
    const res: string[] = [];

    for (const line of this._lines) {
      if (line.kind !== LineKind.Text) {
        continue;
      }

      if (line.tag !== null) {
        const tag = this._tags[line.tag];
        if (tag === undefined || !tag.enabled) {
          continue;
        }
      }

      res.push(line.text);
    }

    return res;
  }

  snippetLines(name: string): [start: number, end: number] | null {
    let start = -1;
    let end = -1;
    let i = 0;
    let lineID = 0;

    for (; i < this._lines.length; i++) {
      const line = this._lines[i];

      if (line.tag !== null) {
        const tag = this._tags[line.tag];
        if (tag === undefined || !tag.enabled) {
          continue;
        }
      }

      if (line.kind === LineKind.Text) {
        lineID++;
      } else if (line.kind === LineKind.TagStart && line.tag === name) {
        start = lineID;
        lineID++;
        break;
      }
    }

    if (start === -1) {
      return null;
    }

    for (; i < this._lines.length; i++) {
      const line = this._lines[i];

      if (line.tag !== null) {
        const tag = this._tags[line.tag];
        if (tag === undefined || !tag.enabled) {
          continue;
        }
      }

      if (line.kind === LineKind.Text) {
        lineID++;
      } else if (line.kind === LineKind.TagEnd && line.tag === name) {
        end = lineID - 1;
        break;
      }
    }

    if (end === -1) {
      return null;
    }

    return [start, end];
  }

  enable(name: string): void {
    if (!this._tags.hasOwnProperty(name)) {
      // TODO.
      return;
    }

    const tag = this._tags[name];

    tag.enabled = true;
    if (tag.replaces !== null) {
      this.disable(tag.replaces);
    }
  }

  disable(name: string): void {
    if (!this._tags.hasOwnProperty(name)) {
      // TODO.
      return;
    }

    const tag = this._tags[name];

    tag.enabled = false;
    if (tag.replaces !== null) {
      this.enable(tag.replaces);
    }
  }
}
