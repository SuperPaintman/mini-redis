'use strict';
/* Imports */
import * as fs from 'fs';
import { join, resolve, extname, dirname, basename } from 'path';
import { promisify, callbackify } from 'util';
import type { Compiler, WebpackPluginInstance } from 'webpack';
import { marked } from 'marked';
import { codeSnippet } from '../lib/marked-code-snippet';
import { FileParser, File, Workspace } from '../lib/splitter';
import * as frontMatter from 'yaml-front-matter';
import * as Prism from 'prismjs';
import 'prismjs/components/prism-go';
import '../lib/prism-redis';
import * as yaml from 'js-yaml';
import * as which from 'which';
import { sync as execaSync } from 'execa';
import glob from 'glob';

/* Helpers */
const readFileAsync = promisify(fs.readFile);
const globAsync = promisify(glob);

function highlight(code: string, lang: string): string {
  const grammar = Prism.languages[lang];

  if (!grammar) {
    return code;
  }

  return Prism.highlight(code, grammar, lang);
}

function format(code: string, lang: string): string {
  if (lang !== 'go') {
    return code;
  }

  const gofmt = which.sync('gofmt');

  const result = execaSync(gofmt, [], {
    input: code
  });
  if (result.exitCode !== 0) {
    throw new Error(result.stderr);
  }

  return result.stdout;
}

/* Types */
type Manifest = {
  chapters: string[];
};

type Chapter = {
  meta: Record<string, unknown>;
  html: string;
};

/* Plugin */
export type Options = {
  manifest?: string;
  workspace?: string;
  marked?: marked.MarkedOptions;
};

export class BookPlugin implements WebpackPluginInstance {
  constructor(private _options: Options = {}) {}

  apply(compiler: Compiler) {
    const pluginName = BookPlugin.name;

    const {
      webpack: {
        Compilation,
        WebpackError,
        sources: { RawSource }
      }
    } = compiler;

    const isProductionLikeMode =
      compiler.options.mode === 'production' || !compiler.options.mode;

    const fileParser = new FileParser();

    const manifestPath = resolve(
      compiler.context,
      this._options.manifest ?? 'book.yml'
    );
    const manifestDirname = dirname(manifestPath);

    // Caches.
    let changed = true;
    let manifest: Manifest | null = null;
    let workspace = new Workspace();
    let chaptersContents: Record<
      string,
      | { type: 'file'; meta: Record<string, unknown>; content: string }
      | { type: 'error'; err: Error }
      | null
    > = {};
    const workspaceFiles: Record<string, File | null> = {};

    compiler.hooks.watchRun.tapAsync(pluginName, (compiler, callback) => {
      // Invalidate files.
      function invalidate(path: string): void {
        if (path === manifestPath) {
          manifest = null;
        } else if (chaptersContents[path]) {
          chaptersContents[path] = null;
        } else if (workspaceFiles[path]) {
          workspaceFiles[path] = null;
        } else {
          return;
        }

        changed = true;
      }

      if (compiler.modifiedFiles) {
        compiler.modifiedFiles.forEach(invalidate);
      }

      if (compiler.removedFiles) {
        compiler.modifiedFiles.forEach(invalidate);
      }

      callback();
    });

    compiler.hooks.beforeCompile.tapAsync(
      pluginName,
      callbackify(async (_params: any) => {
        // Read the manifest.
        if (manifest === null) {
          try {
            const content = await readFileAsync(manifestPath, 'utf-8');

            manifest = yaml.load(content) as Manifest;

            changed = true;
          } catch (e) {}
        }

        // Read chapters.
        if (manifest === null) {
          // Invalidate all.
          for (const path in chaptersContents) {
            if (!chaptersContents.hasOwnProperty(path)) {
              continue;
            }

            chaptersContents[path] = null;

            changed = true;
          }
        } else {
          const seenChapters = new Set<string>();

          for (const chapter of manifest.chapters) {
            const path = resolve(manifestDirname, chapter);

            seenChapters.add(path);

            if (chaptersContents[path]) {
              continue;
            }

            try {
              const md = await readFileAsync(path, 'utf-8');
              const meta = frontMatter.loadFront(md);
              const content = meta.__content;
              delete (meta as any).__content;

              chaptersContents[path] = { type: 'file', meta, content };
            } catch (err) {
              chaptersContents[path] = { type: 'error', err: err as Error };
            }

            changed = true;
          }

          for (const path in chaptersContents) {
            if (!chaptersContents.hasOwnProperty(path)) {
              continue;
            }

            if (!seenChapters.has(path)) {
              chaptersContents[path] = null;

              changed = true;
            }
          }
        }

        // Read workspace files.
        if (this._options.workspace !== undefined) {
          workspace = new Workspace();

          const workspacePattern = resolve(
            compiler.context,
            this._options.workspace
          );

          const files = await globAsync(workspacePattern);

          for (const path of files) {
            const oldFile = workspaceFiles[path];
            if (oldFile) {
              workspace.addFile(oldFile);
              continue;
            }

            const source = await readFileAsync(path, 'utf-8');

            const ext = extname(path).slice(1);

            const file = fileParser.parse(path, ext, source);

            workspace.addFile(file);
            workspaceFiles[path] = file;

            changed = true;
          }
        }

        for (const file of workspace.files()) {
          file.disableAll();
        }
      })
    );

    compiler.hooks.thisCompilation.tap(pluginName, (compilation, params) => {
      const logger = compilation.getLogger(pluginName);

      // Watch the manifest file.
      compilation.fileDependencies.add(manifestPath);

      if (!manifest) {
        compilation.errors.push(new WebpackError('Book manifest not found'));
      }

      // Watch chapters.
      for (const path in chaptersContents) {
        if (!chaptersContents.hasOwnProperty(path)) {
          continue;
        }

        const chapter = chaptersContents[path];
        if (chapter) {
          if (chapter.type === 'file') {
            compilation.fileDependencies.add(path);
          } else {
            compilation.errors.push(new WebpackError('' + chapter.err));
          }
        } else {
          compilation.fileDependencies.delete(path);
          delete chaptersContents[path];
        }
      }

      // Watch workspace files.
      for (const path in workspaceFiles) {
        if (!workspaceFiles.hasOwnProperty(path)) {
          continue;
        }

        if (workspaceFiles[path]) {
          compilation.fileDependencies.add(path);
        } else {
          compilation.fileDependencies.delete(path);
          delete workspaceFiles[path];
        }
      }

      // Skip if nothing was changed.
      // if (!changed) {
      //   logger.debug('Book is not changed');
      //   return;
      // }
      // logger.debug('Book is changed');
      // changed = false;

      // Set-up marked.
      (marked as any).defaults = marked.getDefaults();
      marked.use({
        highlight,
        extensions: [
          codeSnippet(workspace, {
            highlight,
            format: isProductionLikeMode ? format : undefined
          })
        ]
      });

      compilation.hooks.processAssets.tapAsync(
        {
          name: pluginName,
          stage: Compilation.PROCESS_ASSETS_STAGE_ADDITIONAL
        },
        (compilationAssets, cb) => {
          // Compile chapters.
          const chapters: Array<{
            meta: {
              title: string;
              slug: string;
              headings: Array<{ text: string; slug: string }>;
              draft: boolean;
              prev: string | null;
              next: string | null;
            };
            content: string;
          }> = [];

          if (manifest) {
            // Collect chapters meta data.
            const chaptersSlugger = new marked.Slugger();

            for (const chapter of manifest.chapters) {
              const path = resolve(manifestDirname, chapter);

              const cached = chaptersContents[path];
              if (!cached || cached.type !== 'file') {
                continue;
              }

              const { meta, content } = cached;

              const slugger = new marked.Slugger();
              const tokens = marked.Lexer.lex(content, this._options.marked);

              const headings = (tokens as marked.Tokens.Heading[])
                .filter((token) => token.type === 'heading')
                .map((token) => ({
                  text: token.text,
                  level: token.depth,
                  slug: slugger.slug(token.text)
                }));

              const title = headings.find((h) => h.level === 1)?.text;
              const slug =
                (meta.slug as string | undefined) ??
                chaptersSlugger.slug(title ?? basename(path, '.md'));

              chapters.push({
                meta: {
                  title: title ?? slug,
                  slug,
                  headings: headings.filter((h) => h.level === 2),
                  draft: false,
                  next: null,
                  prev: null,
                  ...meta
                },
                content
              });
            }

            // Collect prev/next links.
            for (let i = 0; i < chapters.length; i++) {
              chapters[i].meta.prev = chapters[i - 1]?.meta.slug ?? null;
              chapters[i].meta.next = chapters[i + 1]?.meta.slug ?? null;
            }

            // Build chapters.
            for (const chapter of chapters) {
              const { meta, content } = chapter;

              const html = marked(content, this._options.marked);

              compilation.emitAsset(
                join('api', 'chapters', `${meta.slug}.json`),
                new RawSource(
                  JSON.stringify(
                    {
                      meta,
                      html
                    },
                    null,
                    isProductionLikeMode ? 0 : 2
                  )
                )
              );

              if (!isProductionLikeMode) {
                compilation.emitAsset(
                  join('api', 'chapters', `${meta.slug}.html`),
                  new RawSource(html),
                  {
                    development: true
                  }
                );
              }
            }
          }

          // Index.
          compilation.emitAsset(
            join('api', 'chapters', 'index.json'),
            new RawSource(
              JSON.stringify(
                {
                  chapters: chapters.map(
                    ({ meta: { title, slug, draft } }) => ({
                      title,
                      slug,
                      draft
                    })
                  )
                },
                null,
                isProductionLikeMode ? 0 : 2
              )
            )
          );

          cb();
        }
      );
    });
  }
}
