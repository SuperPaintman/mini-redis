'use strict';
/* Imports */
import { File } from './file';

export class Workspace {
  private _tags: Record<string, File> = {};
  private _files: Record<string, File> = {};

  addFile(file: File): this {
    const tags = file.tags();

    for (const [name, _] of Object.entries(tags)) {
      this._tags[name] = file;
    }

    this._files[file.path()] = file;

    return this;
  }

  file(name: string): File | null {
    if (!this._tags.hasOwnProperty(name)) {
      // TODO.
      return null;
    }

    return this._tags[name];
  }

  files(): File[] {
    return Object.values(this._files);
  }

  enable(name: string): void {
    if (!this._tags.hasOwnProperty(name)) {
      // TODO.
      return;
    }

    this._tags[name].enable(name);
  }

  disable(name: string): void {
    if (!this._tags.hasOwnProperty(name)) {
      // TODO.
      return;
    }

    this._tags[name].disable(name);
  }
}
