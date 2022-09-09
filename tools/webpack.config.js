'use strict';
/* Imports */
require('ts-node').register();

const fs = require('fs');
const path = require('path');

const { DefinePlugin } = require('webpack');
const HtmlWebpackPlugin = require('html-webpack-plugin');
const MiniCssExtractPlugin = require('mini-css-extract-plugin');
const { CleanWebpackPlugin } = require('clean-webpack-plugin');

const sveltePreprocess = require('svelte-preprocess');

const { BookPlugin } = require('./webpack/book-plugin');

/* Init */
const mode = process.env.NODE_ENV || 'development';
const prod = mode === 'production';
const isWebpackDevServer = process.env.WEBPACK_DEV_SERVER === 'true';
const ssr = !isWebpackDevServer;

const srcPath = path.join(__dirname, 'site');
const imagesPath = path.join(srcPath, 'images');
const outputPath = path.join(__dirname, 'public');

/* Helpers */
function maybeCall(fn) {
  return typeof fn === 'function' ? fn() : fn;
}

function filter(array) {
  return array.filter((item) => !!item);
}

function only(isIt, fn, fall) {
  if (!isIt) {
    return fall !== undefined ? maybeCall(fall) : null;
  }

  return maybeCall(fn);
}

const onlyProd = (fn, fall) => only(prod, fn, fall);
const onlyDev = (fn, fall) => only(!prod, fn, fall);

/* Config */
module.exports = {
  mode,
  experiments: {
    layers: true
  },
  entry: {
    main: path.join(srcPath, 'index.ts')
  },
  output: {
    path: outputPath,
    filename: `js/[name]${onlyProd('.[chunkhash]', '')}.js`,
    chunkFilename: `js/[name]${onlyProd('.[chunkhash]', '')}.chunk.js`,
    sourceMapFilename: '[file].map',
    publicPath: '/',
    libraryTarget: 'umd',
    globalObject: "typeof self !== 'undefined' ? self : this"
  },
  devtool: onlyDev('source-map', false),
  resolve: {
    alias: {
      svelte: path.dirname(require.resolve('svelte/package.json')),
      images: imagesPath,
      '~': srcPath
    },
    extensions: ['.ts', '.mjs', '.js', '.svelte'],
    mainFields: ['svelte', 'browser', 'module', 'main']
  },
  plugins: filter([
    /* Define */
    new DefinePlugin({
      IS_SSR: JSON.stringify(ssr)
    }),

    /* Clean */
    new CleanWebpackPlugin(),

    /* HTML */
    new HtmlWebpackPlugin({
      template: path.join(srcPath, 'index.html'),
      excludeChunks: ['server']
    }),

    /* CSS */
    new MiniCssExtractPlugin({
      filename: `css/[name]${onlyProd('.[chunkhash]', '')}.css`,
      chunkFilename: `css/[name]${onlyProd('.[chunkhash]', '')}.chunk.css`
    }),

    /* Book */
    new BookPlugin({
      manifest: path.join(__dirname, 'book', 'manifest.yml'),
      workspace: path.join(__dirname, '..', '{cmd,radish}', '**', '*.go')
    })
  ]),
  module: {
    rules: filter([
      /* TypeScript */
      {
        test: /\.ts$/,
        loader: 'babel-loader',
        exclude: /node_modules/
      },

      /* Svelte */
      {
        test: /\.svelte$/,
        use: {
          loader: 'svelte-loader',
          options: {
            compilerOptions: {
              dev: !prod,
              generate: 'dom',
              hydratable: ssr
            },
            // emitCss: prod,
            emitCss: true, // Responsive loader doesn't work without it.
            hotReload: !prod,
            hotOptions: {
              noPreserveState: false,
              optimistic: true
            },
            preprocess: sveltePreprocess({ sourceMap: !prod })
          }
        }
      },
      {
        // required to prevent errors from Svelte on Webpack 5+
        test: /node_modules\/svelte\/.*\.mjs$/,
        resolve: {
          fullySpecified: false
        }
      },

      /* CSS */
      {
        test: /\.styl$/,
        exclude: /node_modules/,
        use: [
          { loader: MiniCssExtractPlugin.loader },
          { loader: 'css-loader' },
          { loader: 'stylus-loader' }
        ]
      },
      {
        test: /\.css$/,
        use: [{ loader: MiniCssExtractPlugin.loader }, { loader: 'css-loader' }]
      },

      /* Images */
      {
        test: (path) => path.indexOf(imagesPath) === 0,
        use: [
          {
            loader: 'responsive-loader',
            options: {
              name: `images/[name].[width]${onlyProd(
                '.[sha256:hash]',
                ''
              )}.[ext]`
            }
          }
        ]
      }
    ])
  },
  devServer: {
    hot: true,
    // contentBase: outputPath,
    // stats: 'errors-only',
    // watchContentBase: true,
    historyApiFallback: true
  }
};
