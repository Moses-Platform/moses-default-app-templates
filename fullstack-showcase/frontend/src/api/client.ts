interface MosesInfo {
  tenant_id: string;
  user_id: string;
  chart_id: string;
  tool_id: string;
  request_id: string;
  mcp_source: string;
  api_key_id: string;
  headers_present: boolean;
  deployment_mode: 'standalone' | 'mcp-proxy';
}

interface Capability {
  id: string;
  title: string;
  description: string;
  category: string;
  icon: string;
  details: string[];
}

async function fetchAPI<T>(path: string): Promise<T> {
  const response = await fetch(path, {
    headers: {
      'Content-Type': 'application/json',
    },
  });

  if (!response.ok) {
    throw new Error(`API error: ${response.statusText}`);
  }

  return response.json();
}

export async function getMosesInfo(): Promise<MosesInfo> {
  return fetchAPI<MosesInfo>('api/v1/moses-info');
}

export async function getCapabilities(): Promise<Capability[]> {
  return fetchAPI<Capability[]>('api/v1/capabilities');
}

export async function getCapability(id: string): Promise<Capability> {
  return fetchAPI<Capability>(`api/v1/capabilities/${id}`);
}

export async function getHealth(): Promise<{ status: string; service: string; version: string }> {
  return fetchAPI('health');
}
