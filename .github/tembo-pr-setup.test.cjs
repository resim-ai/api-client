const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
const { test } = require('node:test');

const workflow = readFileSync(join(__dirname, 'workflows/tembo-pr-setup.yml'), 'utf8');
const script = workflow.match(/          script: \|\n([\s\S]*)$/)[1]
  .split('\n').map(line => line.slice(12)).join('\n');
const execute = new (Object.getPrototypeOf(async function () {}).constructor)('github', 'context', script);

async function run({ repo = 'rerun', changes = {}, failLabel = false } = {}) {
  const pr = {
    state: 'open',
    user: { login: 'tembo[bot]', type: 'Bot' },
    head: { repo: { full_name: `resim-ai/${repo}` }, sha: 'current-head' },
    base: { ref: 'main' },
    draft: false,
    labels: [],
    auto_merge: null,
    node_id: 'PR_test',
    ...changes,
  };
  const writes = [];
  const github = {
    rest: {
      pulls: { get: async () => ({ data: pr }) },
      issues: { addLabels: async input => {
        if (failLabel) throw new Error('label failed');
        writes.push({ kind: 'labels', ...input });
      } },
    },
    graphql: async (query, variables) => {
      assert.match(query, /enablePullRequestAutoMerge\(input: \$input\)/);
      writes.push({ kind: 'autoMerge', ...variables });
    },
  };
  await execute(github, { repo: { owner: 'resim-ai', repo }, payload: { number: 42 } });
  return writes;
}

const label = { kind: 'labels', owner: 'resim-ai', repo: 'rerun', issue_number: 42, labels: ['ready'] };
const autoMerge = { kind: 'autoMerge', input: { pullRequestId: 'PR_test', mergeMethod: 'SQUASH', expectedHeadOid: 'current-head' } };

test('rerun bot PR gets ready and native squash auto-merge', async () => {
  assert.deepEqual(await run(), [label, autoMerge]);
});

test('rerun drafts get ready without leaving draft or enabling auto-merge', async () => {
  assert.deepEqual(await run({ changes: { draft: true } }), [label]);
});

for (const repo of ['gotta-go-fast', 'api-client']) {
  test(`${repo} bot PR gets auto-merge without ready`, async () => {
    assert.deepEqual(await run({ repo }), [autoMerge]);
    assert.deepEqual(await run({ repo, changes: { draft: true } }), []);
  });
}

test('existing labels are preserved and ready is not re-added', async () => {
  assert.deepEqual(await run({ changes: { labels: [{ name: 'tembo' }] } }), [label, autoMerge]);
  assert.deepEqual(await run({ changes: { labels: [{ name: 'ready' }] } }), [autoMerge]);
});

test('existing auto-merge is preserved', async () => {
  assert.deepEqual(await run({ changes: { auto_merge: { merge_method: 'squash' } } }), [label]);
});

for (const [name, changes] of Object.entries({
  human: { user: { login: 'pete-resim', type: 'User' } },
  'non-bot identity': { user: { login: 'tembo[bot]', type: 'User' } },
  fork: { head: { repo: { full_name: 'elsewhere/rerun' }, sha: 'fork-head' } },
  'deleted fork': { head: { repo: null, sha: 'fork-head' } },
  'other base': { base: { ref: 'release' } },
  closed: { state: 'closed' },
})) {
  test(`${name} PR gets no metadata writes`, async () => {
    assert.deepEqual(await run({ changes }), []);
  });
}

test('label failure stops before enabling auto-merge', async () => {
  await assert.rejects(run({ failLabel: true }), /label failed/);
});
