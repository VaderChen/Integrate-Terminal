const { test } = require('node:test');
const assert = require('node:assert/strict');
const { sourceLoader } = require('./load-source.cjs');
const { mcpClientConfig } = sourceLoader({ globals: { URL } })('src/components/mcpSettings.ts');

test('127.0.0.1 MCP 設定免金鑰，其他網址保留驗證', () => {
  const config = endpoint => JSON.parse(mcpClientConfig('network', '', endpoint)).mcpServers.integterm;
  assert.deepEqual(config('http://127.0.0.1:34115/mcp'), { url: 'http://127.0.0.1:34115/mcp' });
  for (const endpoint of ['http://192.0.2.1/mcp', 'http://127.0.0.1.evil.example/mcp', 'http://[::1]/mcp']) {
    assert.equal(config(endpoint).headers.Authorization, 'Bearer <YOUR_API_TOKEN>');
  }
});
