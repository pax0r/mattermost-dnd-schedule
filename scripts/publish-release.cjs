const fs = require('node:fs/promises');
const path = require('node:path');
const crypto = require('node:crypto');

// GitHub supplies the client and context. Tests inject a fake client; no network
// call occurs until the artifact, tag, checksum, and notes have been checked.
module.exports = async function publishRelease({github, context, tag, commit, directory = 'dist'}) {
    const files = await fs.readdir(directory);
    const bundles = files.filter((name) => name.endsWith('.tar.gz'));
    if (bundles.length !== 1) {
        throw new Error('Expected exactly one plugin bundle');
    }
    const bundle = bundles[0];
    const checksum = bundle + '.sha256';
    const payload = await fs.readFile(path.join(directory, bundle));
    const expected = crypto.createHash('sha256').update(payload).digest('hex') + '  ' + bundle + '\n';
    if (await fs.readFile(path.join(directory, checksum), 'utf8') !== expected) {
        throw new Error('Checksum mismatch');
    }
    if (typeof tag !== 'string' || !/^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$/.test(tag)) {
        throw new Error('Publishing requires an explicit version tag');
    }
    if (typeof commit !== 'string' || !/^[0-9a-f]{40}$/.test(commit)) {
        throw new Error('Publishing requires the exact release commit');
    }
    if (!bundle.endsWith('-' + tag.slice(1) + '.tar.gz')) {
        throw new Error('Bundle filename does not match the release tag');
    }
    const body = await fs.readFile(path.join(directory, 'release-notes.md'), 'utf8');
    const {owner, repo} = context.repo;
    const {data: release} = await github.rest.repos.createRelease({
        owner, repo, tag_name: tag, target_commitish: commit, name: 'Scheduled DND ' + tag,
        body, draft: true, prerelease: tag.includes('-'),
    });
    for (const name of [bundle, checksum]) {
        await github.rest.repos.uploadReleaseAsset({
            owner, repo, release_id: release.id, name,
            headers: {'content-type': name.endsWith('.tar.gz') ? 'application/gzip' : 'text/plain'},
            data: await fs.readFile(path.join(directory, name)),
        });
    }
    await github.rest.repos.updateRelease({owner, repo, release_id: release.id, draft: false});
};
