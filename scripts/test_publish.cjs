const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');

const publishRelease = require('./publish-release.cjs');

async function fixture(t, version = '0.1.0', {failChecksumUpload = false} = {}) {
    const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'dnd-release-'));
    t.after(() => fs.rm(directory, {recursive: true, force: true}));
    const name = `example.dnd-${version}.tar.gz`;
    // The publisher transports bytes; the build checks real archive structure.
    const payload = Buffer.from('release asset test fixture');
    const checksum = crypto.createHash('sha256').update(payload).digest('hex');
    await fs.writeFile(path.join(directory, name), payload);
    await fs.writeFile(path.join(directory, name + '.sha256'), `${checksum}  ${name}\n`);
    await fs.writeFile(path.join(directory, 'release-notes.md'), 'Installation instructions');
    const calls = [];
    const github = {rest: {repos: {
        async createRelease(input) { calls.push({method: 'create', input}); return {data: {id: 42}}; },
        async uploadReleaseAsset(input) {
            calls.push({method: 'upload', input});
            if (failChecksumUpload && input.name.endsWith('.sha256')) { throw new Error('upload failed'); }
        },
        async updateRelease(input) { calls.push({method: 'publish', input}); },
    }}};
    return {directory, github, tag: `v${version}`, commit: 'a'.repeat(40), context: {ref: 'refs/heads/main', repo: {owner: 'example', repo: 'dnd'}}, calls, name};
}

test('publishes only after both assets upload, preserving their contents and types', async (t) => {
    const setup = await fixture(t);
    await publishRelease(setup);
    assert.deepEqual(setup.calls.map((call) => call.method), ['create', 'upload', 'upload', 'publish']);
    assert.equal(setup.calls[0].input.draft, true);
    assert.equal(setup.calls[0].input.prerelease, false);
    assert.equal(setup.calls[0].input.tag_name, 'v0.1.0');
    assert.equal(setup.calls[0].input.target_commitish, setup.commit);
    assert.equal(setup.calls[1].input.name, setup.name);
    assert.equal(setup.calls[1].input.headers['content-type'], 'application/gzip');
    assert.equal(setup.calls[2].input.headers['content-type'], 'text/plain');
    assert.equal(setup.calls[3].input.draft, false);
});

test('upload failure leaves the release unpublished', async (t) => {
    const setup = await fixture(t, '0.1.0', {failChecksumUpload: true});
    await assert.rejects(publishRelease(setup), /upload failed/);
    assert.deepEqual(setup.calls.map((call) => call.method), ['create', 'upload', 'upload']);
});

test('corrupt checksum or wrong tag cannot create a release', async (t) => {
    const badChecksum = await fixture(t);
    await fs.writeFile(path.join(badChecksum.directory, badChecksum.name + '.sha256'), 'wrong');
    await assert.rejects(publishRelease(badChecksum), /Checksum mismatch/);
    assert.equal(badChecksum.calls.length, 0);
    const badTag = await fixture(t);
    badTag.tag = 'v0.2.0';
    await assert.rejects(publishRelease(badTag), /does not match/);
    assert.equal(badTag.calls.length, 0);
});

test('release candidates are published as prereleases', async (t) => {
    const setup = await fixture(t, '0.2.0-rc.1');
    await publishRelease(setup);
    assert.equal(setup.calls[0].input.prerelease, true);
});

test('a branch ref cannot implicitly select a publication tag', async (t) => {
    const setup = await fixture(t);
    delete setup.tag;
    await assert.rejects(publishRelease(setup), /explicit version tag/);
    assert.equal(setup.calls.length, 0);
});

test('publication cannot fall back to an unspecified source commit', async (t) => {
    const setup = await fixture(t);
    delete setup.commit;
    await assert.rejects(publishRelease(setup), /exact release commit/);
    assert.equal(setup.calls.length, 0);
});
