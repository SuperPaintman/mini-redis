'use strict';
/* Imports */
import * as fs from 'fs';
import { join, relative } from 'path';
import { promisify } from 'util';
import glob from 'glob';
import { marked } from 'marked';
import * as Prism from 'prismjs';
import 'prismjs/components/prism-go';
import './lib/prism-redis';
import { codeSnippet } from './lib/marked-code-snippet';
import { Workspace, FileParser } from './lib/splitter';
import Parser from 'tree-sitter';
import * as which from 'which';
import { sync as execaSync } from 'execa';

/* Helpers */
const readFileAsync = promisify(fs.readFile);
const writeFileAsync = promisify(fs.writeFile);
const globAsync = promisify(glob);
const quote = (s: string): string => JSON.stringify(s);
const top = <T>(arr: T[]): T | null =>
  arr.length > 0 ? arr[arr.length - 1] : null;

/* Init */
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

    const file = fileParser.parse(filename, 'go', content);

    workspace.addFile(file);
  }

  workspace.enable('Redis Protocol');

  marked.use({
    extensions: [
      codeSnippet(workspace, {
        highlight(code, lang) {
          const grammar = Prism.languages[lang];

          if (!grammar) {
            return code;
          }

          return Prism.highlight(code, grammar, lang);
        },
        format(code, lang) {
          if (lang !== 'go') {
            return code;
          }

          const gofmt = which.sync('gofmt');

          const result = execaSync(gofmt, [], {
            input: code
          });

          return result.stdout;
        }
      })
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
}

if (!module.parent) {
  main().catch((err) => {
    console.error(err);
    process.exit(1);
  });
}
