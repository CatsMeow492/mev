import { NextRequest, NextResponse } from 'next/server';

export const runtime = 'edge';

export async function GET(request: NextRequest) {
  const { searchParams } = new URL(request.url);
  const path = searchParams.get('path') || '';
  
  // Get the target WebSocket URL from environment
  const wsUrl = process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080';
  const targetUrl = `${wsUrl}/ws/${path}`;
  
  // For WebSocket connections, we need to upgrade the connection
  // In Edge Functions, we can't directly handle WebSocket upgrades
  // Instead, we'll return connection details for the client to connect directly
  
  const headers = new Headers();
  headers.set('Access-Control-Allow-Origin', '*');
  headers.set('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
  headers.set('Access-Control-Allow-Headers', 'Content-Type, Authorization');
  
  // Return WebSocket connection information
  return NextResponse.json({
    websocketUrl: targetUrl,
    protocol: 'websocket',
    timestamp: new Date().toISOString(),
    path: path
  }, {
    status: 200,
    headers: headers
  });
}

export async function POST(request: NextRequest) {
  const { searchParams } = new URL(request.url);
  const path = searchParams.get('path') || '';
  
  // Handle WebSocket authentication and connection setup
  const body = await request.json();
  
  // Validate authentication token if provided
  const authToken = request.headers.get('Authorization');
  if (authToken && !validateAuthToken(authToken)) {
    return NextResponse.json(
      { error: 'Invalid authentication token' },
      { status: 401 }
    );
  }
  
  const wsUrl = process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080';
  const targetUrl = `${wsUrl}/ws/${path}`;
  
  // Return authenticated WebSocket connection details
  const headers = new Headers();
  headers.set('Access-Control-Allow-Origin', '*');
  headers.set('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
  headers.set('Access-Control-Allow-Headers', 'Content-Type, Authorization');
  
  return NextResponse.json({
    websocketUrl: targetUrl,
    protocol: 'websocket',
    authenticated: !!authToken,
    timestamp: new Date().toISOString(),
    path: path,
    connectionId: generateConnectionId()
  }, {
    status: 200,
    headers: headers
  });
}

export async function OPTIONS(request: NextRequest) {
  const headers = new Headers();
  headers.set('Access-Control-Allow-Origin', '*');
  headers.set('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
  headers.set('Access-Control-Allow-Headers', 'Content-Type, Authorization');
  
  return new Response(null, {
    status: 200,
    headers: headers
  });
}

function validateAuthToken(token: string): boolean {
  // Basic token validation - in production, this should validate JWT tokens
  if (!token || !token.startsWith('Bearer ')) {
    return false;
  }
  
  // Extract token without "Bearer " prefix
  const actualToken = token.slice(7);
  
  // Add your JWT validation logic here
  // For now, just check if token exists and has minimum length
  return actualToken.length > 10;
}

function generateConnectionId(): string {
  return `conn_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
} 