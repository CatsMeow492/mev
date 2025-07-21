'use client';

import { useState, useEffect } from 'react';
import { useApi } from '@/hooks/useApi';

export default function TestPage() {
  const [status, setStatus] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);
  const { getSystemStatus } = useApi();

  const testConnection = async () => {
    try {
      const response = await fetch('https://mev-strategy-dev.fly.dev/api/v1/status');
      const data = await response.json();
      setStatus(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Connection failed');
      setStatus(null);
    }
  };

  useEffect(() => {
    testConnection();
  }, []);

  return (
    <div className="min-h-screen bg-gray-900 text-white p-8">
      <h1 className="text-3xl font-bold mb-8">MEV Engine Connection Test</h1>
      
      <div className="space-y-6">
        <button
          onClick={testConnection}
          className="px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded"
        >
          Test Backend Connection
        </button>

        {error && (
          <div className="p-4 bg-red-600 rounded">
            <h2 className="font-bold">Error:</h2>
            <p>{error}</p>
          </div>
        )}

        {status && (
          <div className="p-4 bg-green-600 rounded">
            <h2 className="font-bold mb-4">✅ Backend Connected Successfully!</h2>
            <pre className="bg-gray-800 p-4 rounded overflow-x-auto">
              {JSON.stringify(status, null, 2)}
            </pre>
          </div>
        )}

        <div className="mt-8">
          <h2 className="text-xl font-bold mb-4">Connection Details:</h2>
          <ul className="space-y-2">
            <li>Backend URL: https://mev-strategy-dev.fly.dev</li>
            <li>Frontend URL: dashboard-brand-beacon.vercel.app</li>
            <li>API Endpoint: /api/v1/status</li>
            <li>WebSocket: wss://mev-strategy-dev.fly.dev</li>
          </ul>
        </div>
      </div>
    </div>
  );
} 