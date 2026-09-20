export const MCP_TOKEN_PLACEHOLDER = '<YOUR_API_TOKEN>';

export function redactMCPToken(markdown: string, token: string): string {
  return token ? markdown.split(token).join(MCP_TOKEN_PLACEHOLDER) : markdown;
}

export function mcpClientConfig(contract: 'local' | 'network', executable: string, endpoint: string): string {
  return JSON.stringify({
    mcpServers: {
      integterm: contract === 'local'
        ? { command: executable, args: ['mcp'] }
        : { url: endpoint, headers: { Authorization: `Bearer ${MCP_TOKEN_PLACEHOLDER}` } },
    },
  }, null, 2);
}

export function parseMCPPort(value: string): number | null {
  if (!/^\d+$/.test(value.trim())) return null;
  const port = Number(value.trim());
  return Number.isInteger(port) && port > 0 && port <= 65535 ? port : null;
}
