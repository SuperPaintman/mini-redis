'use strict';
/* Imports */
import * as fs from 'fs';
import { join, relative } from 'path';
import { promisify } from 'util';
import glob from 'glob';
import { marked } from 'marked';
import * as Prism from 'prismjs';
import 'prismjs/components/prism-go';
import Parser from 'tree-sitter';
const Golang = require('tree-sitter-go');

/* Helpers */
const readFileAsync = promisify(fs.readFile);
const writeFileAsync = promisify(fs.writeFile);
const globAsync = promisify(glob);
const quote = (s: string): string => JSON.stringify(s);
const top = <T>(arr: T[]): T | null =>
  arr.length > 0 ? arr[arr.length - 1] : null;
const has = (obj: object, key: string) => obj.hasOwnProperty(key);

/* Init */
Prism.languages.redis = {
  number: {
    pattern: /(\$|\*|:)\-?\d+(?:\\r\\n)/,
    inside: {
      keyword: /^(\$|\*|:)/,
      punctuation: /\\r\\n$/
    }
  },
  string: [
    {
      pattern: /(\+|-).+(?:\\r\\n)/,
      inside: {
        keyword: /^(\+|-)/,
        punctuation: /\\r\\n$/
      }
    },
    {
      pattern: /.+(?:\\r\\n)/,
      inside: {
        punctuation: /\\r\\n$/
      }
    }
  ],
  punctuation: /\\r\\n/
};

type Tag = {
  enabled: boolean;
  name: string;
  replaces: string | null;
  uncommentLines: string | boolean;
};

enum LineKind {
  Text,
  TagStart,
  TagEnd
}

type Line =
  | {
      kind: LineKind.Text;
      text: string;
      tag: string | null;
    }
  | {
      kind: LineKind.TagStart;
      tag: string;
    }
  | {
      kind: LineKind.TagEnd;
      tag: string;
    };

type MagicLine = {
  marker: MagicMarker;
  kind: string;
  args: string[];
  options: Record<string, string | null>;
};

class File {
  constructor(
    private _path: string,
    private _lines: Line[],
    private _tags: Record<string, Tag>
  ) {}

  path(): string {
    return this._path;
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
    if (!has(this._tags, name)) {
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
    if (!has(this._tags, name)) {
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

enum MagicMarker {
  Start = '>',
  End = '<',
  Line = '^'
}

class MagicLineParser {
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

    if (buf.length === 0) {
      throw new Error('Empty string');
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

function skipWhitespaces(line: string, start: number): number {
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

class FileParser {
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

              if (has(magicLine.options, 'uncomment-lines')) {
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

              if (has(magicLine.options, 'before')) {
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

              if (has(magicLine.options, 'after')) {
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

function uncommentLine(line: string): string {
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

class Workspace {
  private _tags: Record<string, File> = {};
  private _files: Record<string, File> = {};

  constructor() {}

  addFile(file: File): this {
    const tags = file.tags();

    for (const [name, _] of Object.entries(tags)) {
      this._tags[name] = file;
    }

    this._files[file.path()] = file;

    return this;
  }

  file(name: string): File | null {
    if (!has(this._tags, name)) {
      // TODO.
      return null;
    }

    return this._tags[name];
  }

  enable(name: string): void {
    if (!has(this._tags, name)) {
      // TODO.
      return;
    }

    this._tags[name].enable(name);
  }

  disable(name: string): void {
    if (!has(this._tags, name)) {
      // TODO.
      return;
    }

    this._tags[name].disable(name);
  }
}

type GoDeclaration = (
  | {
      kind: 'package';
    }
  | {
      kind: 'func';
      name: string;
      parent: GoDeclaration;
    }
  | {
      kind: 'method';
      name: string;
      receiver: string;
      parent: GoDeclaration;
    }
  | {
      kind: 'type';
      name: string;
      parent: GoDeclaration;
    }
  | {
      kind: 'const';
      name: string;
      parent: GoDeclaration;
    }
  | {
      kind: 'var';
      name: string;
      parent: GoDeclaration;
    }
) & {
  level: number;
  children: GoDeclaration[];
  start: Parser.Point;
  end: Parser.Point;
};

class GoContext {
  constructor(
    readonly root: GoDeclaration,
    private _declarations: GoDeclaration[]
  ) {}
}

class GoContextVisitor {
  private _declarations: GoDeclaration[] = [];
  private _stack: GoDeclaration[] = [];

  constructor() {}

  visit(node: Parser.SyntaxNode): GoContext {
    this._reset();

    assertNodeType(node, 'source_file');

    const pkg: GoDeclaration = {
      kind: 'package',
      children: [],
      level: 0,
      start: node.startPosition,
      end: node.endPosition
    };
    this._declarations.push(pkg);
    this._stack.push(pkg);

    this._visit(node);

    const root = this._stack.pop();
    if (root === undefined || root.kind !== 'package') {
      throw new Error('Broken Go declaration stack');
    }

    return new GoContext(root, this._declarations);
  }

  private _reset(): void {
    this._declarations = [];
    this._stack = [];
  }

  private _visit(node: Parser.SyntaxNode) {
    switch (node.type) {
      case 'function_declaration':
        this._visitFunctionDeclaration(node);
        break;

      case 'method_declaration':
        this._visitMethodDeclaration(node);
        break;

      case 'type_spec':
        this._visitTypeSpec(node);
        break;

      case 'const_spec':
        this._visitConstSpec(node);
        break;

      case 'var_spec':
        this._visitVarSpec(node);
        break;

      default:
        this._visitChildren(node);
        break;
    }
  }

  private _visitFunctionDeclaration(node: Parser.SyntaxNode): void {
    assertNodeType(node.child(0), 'func');
    const identifier = assertNodeType(node.child(1), 'identifier');
    assertNodeType(node.child(2), 'parameter_list');

    let retType: Parser.SyntaxNode | null = node.child(3);
    let block: Parser.SyntaxNode | null = node.child(4);
    if (retType !== null && retType.type == 'block') {
      block = retType;
      retType = null;
    }

    const decl: GoDeclaration = {
      kind: 'func',
      name: identifier.text,
      parent: top(this._stack)!,
      children: [],
      level: this._stack.length,
      start: node.startPosition,
      end: node.endPosition
    };
    top(this._stack)!.children.push(decl);
    this._declarations.push(decl);
    this._stack.push(decl);

    if (block !== null) {
      assertNodeType(block, 'block');

      this._visitChildren(block);
    }

    this._stack.pop();
  }

  private _visitMethodDeclaration(node: Parser.SyntaxNode): void {
    assertNodeType(node.child(0), 'func');
    const receiver = assertNodeType(node.child(1), 'parameter_list');
    const identifier = assertNodeType(node.child(2), 'field_identifier');
    assertNodeType(node.child(3), 'parameter_list');

    let retType: Parser.SyntaxNode | null = node.child(4);
    let block: Parser.SyntaxNode | null = node.child(5);
    if (retType !== null && retType.type == 'block') {
      block = retType;
      retType = null;
    }

    assertNodeType(receiver.child(0), '(');
    const receiverInner = assertNodeType(
      receiver.child(1),
      'parameter_declaration'
    );
    assertNodeType(receiver.child(2), ')');

    let receiverIdentifier = receiverInner.child(0);
    let receiverType = receiverInner.child(1);
    if (receiverType === null) {
      receiverType = receiverIdentifier;
      receiverIdentifier = null;
    }

    receiverType = assertNodeType(receiverType);

    const decl: GoDeclaration = {
      kind: 'method',
      name: identifier.text,
      receiver: receiverType.text,
      parent: top(this._stack)!,
      level: this._stack.length,
      children: [],
      start: node.startPosition,
      end: node.endPosition
    };
    top(this._stack)!.children.push(decl);
    this._declarations.push(decl);
    this._stack.push(decl);

    if (block !== null) {
      assertNodeType(block, 'block');

      this._visitChildren(block);
    }

    this._stack.pop();
  }

  private _visitTypeSpec(node: Parser.SyntaxNode): void {
    const identifier = assertNodeType(node.child(0), 'type_identifier');

    const decl: GoDeclaration = {
      kind: 'type',
      name: identifier.text,
      parent: top(this._stack)!,
      level: this._stack.length,
      children: [],
      start: node.startPosition,
      end: node.endPosition
    };
    top(this._stack)!.children.push(decl);
    this._declarations.push(decl);
    this._stack.push(decl);

    for (const child of node.children.slice(1)) {
      this._visit(child);
    }

    this._stack.pop();
  }

  private _visitConstSpec(node: Parser.SyntaxNode): void {
    const identifier = assertNodeType(node.child(0), 'identifier');

    const decl: GoDeclaration = {
      kind: 'const',
      name: identifier.text,
      parent: top(this._stack)!,
      level: this._stack.length,
      children: [],
      start: node.startPosition,
      end: node.endPosition
    };
    top(this._stack)!.children.push(decl);
    this._declarations.push(decl);
    this._stack.push(decl);

    for (const child of node.children.slice(1)) {
      this._visit(child);
    }

    this._stack.pop();
  }

  private _visitVarSpec(node: Parser.SyntaxNode): void {
    const identifier = assertNodeType(node.child(0), 'identifier');

    const decl: GoDeclaration = {
      kind: 'var',
      name: identifier.text,
      parent: top(this._stack)!,
      level: this._stack.length,
      children: [],
      start: node.startPosition,
      end: node.endPosition
    };
    top(this._stack)!.children.push(decl);
    this._declarations.push(decl);
    this._stack.push(decl);

    for (const child of node.children.slice(1)) {
      this._visit(child);
    }

    this._stack.pop();
  }

  private _visitChildren(node: Parser.SyntaxNode): void {
    for (const child of node.children) {
      this._visit(child);
    }
  }
}

function assertNodeType(
  node: Parser.SyntaxNode | null,
  type: string | string[] | undefined = undefined
): Parser.SyntaxNode {
  if (type === undefined) {
    if (node === null) {
      throw new Error(`Expected any non-null node, got null node`);
    }

    return node;
  }

  const expected =
    typeof type === 'string' ? quote(type) : type.map(quote).join(' or ');

  if (node === null) {
    throw new Error(`Expected ${expected}, got null node`);
  }

  const ok =
    typeof type === 'string'
      ? node.type === type
      : type.some((t) => node.type === t);

  if (!ok) {
    throw new Error(`Expected ${expected}, got ${quote(node.type)}`);
  }

  return node;
}

async function main() {
  const sourceFiles = await globAsync(
    join(__dirname, '..', '{cmd,radish}', '**', '*.go')
  );

  const workspace = new Workspace();
  const fileParser = new FileParser();

  for (const path of sourceFiles) {
    const content = await readFileAsync(path, 'utf-8');

    const filename = relative(join(__dirname, '..'), path);

    const file = fileParser.parse(filename, content);

    workspace.addFile(file);
  }

  workspace.enable('Redis Protocol');
  // project.enable('radish-cli');
  // project.enable('radish-cli-writer');
  // project.enable('radish-cli-import-radish');
  // project.enable('radish-cli-readall');
  // project.enable('radish-cli-import-ioutil');
  // project.enable('radish-cli-reader');
  // project.enable('radish-cli-import-ioutil-remove');
  // project.enable('radish-cli-read-response');
  // project.enable('radish-cli-read-response-array');
  // project.enable('radish-cli-read-response-array-import-math');
  // project.enable('radish-cli-read-response-array-import-str');

  // const res = Prism.highlight(snippet, Prism.languages.go, 'go');

  // console.log(res.split('\n'));

  marked.use({
    extensions: [
      {
        name: 'codeSnippet',
        level: 'block',
        start: (src) => src.match(/\^snippet\s/)?.index,
        tokenizer(src, tokens) {
          const match = src.match(/^(\^snippet\s+.+)(?:\n|$)/);
          if (!match) {
            return;
          }

          const magicLine = new MagicLineParser(match[1].slice(1)).parse(
            MagicMarker.Line
          );
          if (!magicLine) {
            return;
          }

          const token = {
            type: 'codeSnippet',
            raw: match[1],
            ...magicLine,
            tokens: []
          };

          return token;
        },
        renderer(token: marked.Tokens.Generic) {
          const tok = token as marked.Tokens.Generic & MagicLine;

          switch (tok.kind) {
            case 'snippet': {
              const name = tok.args[0];
              if (!name) {
                // TODO
                return tok.raw;
              }

              workspace.enable(name);

              const file = workspace.file(name);
              if (file === null) {
                // TODO
                return tok.raw;
              }

              const pos = file.snippetLines(name);
              if (!pos) {
                // TODO
                return tok.raw;
              }
              const [start, end] = pos;

              const before = parseInt(tok.options.before ?? '0', 10);
              if (isNaN(before)) {
                throw new Error('Broken before options');
              }

              const after = parseInt(tok.options.after ?? '0', 10);
              if (isNaN(after)) {
                throw new Error('Broken after options');
              }

              const lines = file.lines();

              const snippet = lines
                .slice(start - before, end + after)
                .join('\n');

              const code = Prism.highlight(snippet, Prism.languages.go, 'go');

              let res: string = '';

              if (before === 0 && after === 0) {
                res += `<pre><code class="language-go">${code}</code></pre>`;
              } else {
                const codeLines = code.split('\n');
                const linesBefore = codeLines.slice(0, before);
                const linesHighlighted = codeLines.slice(before, -1 * after);
                const linesAfter = codeLines.slice(-1 * after);

                res += '<pre><code class="language-go">';
                if (linesBefore.length > 0) {
                  res += `<div class="dimmed">${linesBefore.join(
                    '\n'
                  )}\n</div>`;
                }

                res += `<div class="highlighted">${linesHighlighted.join(
                  '\n'
                )}\n</div>`;

                if (linesAfter.length > 0) {
                  res += `<div class="dimmed">${linesAfter.join('\n')}\n</div>`;
                }

                res += '</pre></code>';
              }

              res += `<div><em>${file.path()}</em></div>`;

              return res;
            }

            default:
              throw new Error(`Unknown magic line kind: ${quote(tok.kind)}`);
          }
        }
      }
    ]
  });

  marked.setOptions({
    gfm: true,
    highlight(code, lang) {
      const grammar = Prism.languages[lang];

      if (!grammar) {
        return code;
      }

      return Prism.highlight(code, grammar, lang);
    }
  });

  const res = marked.parse(`

<style>

pre {
  background-color: #faf8f5;
}

pre > code {
  text-shadow: none !important;
}

pre > code .dimmed,
pre > code .dimmed .token {
  color: #bab8b7 !important;
}

pre > code .highlighted {
  margin: -2px -12px;
  padding: 2px 10px;

  border-left: solid 2px #dad8d6;
  border-right: solid 2px #dad8d6;

  background-color: #f5f3f0;
}

</style>

<link href="https://cdn.jsdelivr.net/npm/prismjs@1.29.0/themes/prism.css" rel="stylesheet" />

# Hello

~~~go
// HI
~~~

~~~redis
*2\\r\\n
$3\\r\\n
GET\\r\\n
$5\\r\\n
hello\\r\\n

$-1\\r\\n

+OK\\r\\n
:1337\\r\\n
+1337\\r\\n
-ERR Something went wrong\\r\\n


*2\\r\\n$3\\r\\nGET\\r\\n$5\\r\\nhello\\r\\n+OK\\r\\n*2\\r\\n$3\\r\\nGET\\r\\n$5\\r\\nhello\\r\\n
~~~

^snippet radish-cli

^snippet radish-cli-writer: before=1, after=1

^snippet radish-cli-import-radish: before=1, after=1

^snippet radish-cli-readall: before=2, after=1

^snippet radish-cli-import-ioutil: before=1, after=1

^snippet radish-cli-reader: before=2, after=1

^snippet radish-cli-import-ioutil-remove

^snippet radish-cli-read-response

^snippet radish-cli-read-response-array: before=3, after=2

# Test

^snippet radish-cli-read-response-array-import-math: before=1, after=1

# Test

^snippet radish-cli-read-response-array-import-str: before=1, after=2

# Writer

^snippet writer

^snippet writer-data-types

^snippet writer-write-simple-string

^snippet writer-write-type

^snippet writer-write-string

^snippet writer-write-terminator

^snippet writer-error

^snippet writer-write-error

^snippet writer-write-ints

^snippet writer-write-int

^snippet writer-import-strconv: before=1, after=1

^snippet writer-writer-smallbuf-field: before=2, after=1

^snippet writer-writer-smallbuf-size: before=1, after=1

^snippet writer-writer-smallbuf-init: before=2, after=2

^snippet writer-write-uints

^snippet writer-write-uint

^snippet writer-write-bulk

^snippet writer-write-prefix

^snippet writer-write-null

^snippet writer-write-array

# Reader

^snippet reader

^snippet reader-command

^snippet reader-read-command

^snippet reader-read-command-cmd: before=1, after=2

^snippet reader-read-command-array-length: before=3, after=2

^snippet reader-errors

^snippet reader-import-errors: before=1, after=1

^snippet reader-read-value

^snippet reader-error-line-limit-exceeded: before=1, after=1

^snippet reader-read-line

^snippet reader-has-terminator

^snippet reader-parse-int

^snippet reader-read-command-parse-elements: before=3, after=2

^snippet reader-error-bulk-length: before=1, after=2

^snippet reader-read-bulk

^snippet reader-command-grow

^snippet reader-command-pool

^snippet reader-import-sync: before=1, after=1

^snippet reader-read-command-from-pool: before=1, after=4

^snippet reader-read-command-preallocate-args: before=3, after=2

^snippet reader-read-simple-string

^snippet reader-read-error

^snippet reader-read-integer

^snippet reader-error-integer-value: before=1, after=2

^snippet reader-read-string

^snippet reader-read-array

^snippet reader-read-any

^snippet reader-import-fmt: before=1, after=1

^snippet writer-data-type-null: before=1, after=1

hello
`);

  // console.log(res);

  await writeFileAsync(
    join(__dirname, 'res.html'),
    `<html><body>${res}</body></html>`
  );

  const parser = new Parser();
  parser.setLanguage(Golang);

  // const tree = parser.parse(file.lines().join('\n'));
  // const tree = parser.parse(tmpSrc);

  // console.log(tree.rootNode.type);
  //
  // const visitor = new GoContextVisitor();
  // const ctx = visitor.visit(tree.rootNode);

  // console.dir(ctx.root, { depth: 10 });
  //

  // const lines = file.lines();
  // for (let i = 0; i < lines.length; i++) {
  //   const line = lines[i];

  //   console.log(` ${i} | `, line);
  // }

  // const [start, end] = file.snippetLines(
  //   'radish-cli-read-response-array-import-math'
  // ) ?? [-1, -1];
  // console.log(start, end);

  // console.log(tree.rootNode.toString());
}

if (!module.parent) {
  main().catch((err) => {
    console.error(err);
    process.exit(1);
  });
}
