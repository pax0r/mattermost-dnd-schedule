import {build} from 'esbuild';

await build({
    entryPoints: ['src/index.tsx'],
    outfile: 'dist/main.js',
    bundle: true,
    minify: true,
    format: 'iife',
    target: ['es2020'],
    plugins: [{
        name: 'mattermost-host-react',
        setup(builder) {
            builder.onResolve({filter: /^react$/}, () => ({path: 'react', namespace: 'host'}));
            builder.onLoad({filter: /.*/, namespace: 'host'}, () => ({contents: 'module.exports = window.React;', loader: 'js'}));
        },
    }],
});
