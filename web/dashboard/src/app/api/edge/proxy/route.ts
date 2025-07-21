import { NextRequest, NextResponse } from 'next/server';

export const runtime = 'edge';

export async function GET(request: NextRequest) {
  return handleProxyRequest(request, 'GET');
}

export async function POST(request: NextRequest) {
  return handleProxyRequest(request, 'POST');
}

export async function PUT(request: NextRequest) {
  return handleProxyRequest(request, 'PUT');
}

export async function DELETE(request: NextRequest) {
  return handleProxyRequest(request, 'DELETE');
}

export async function OPTIONS(request: NextRequest) {
  const headers = new Headers();
  headers.set('Access-Control-Allow-Origin', '*');
  headers.set('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
  headers.set('Access-Control-Allow-Headers', 'Content-Type, Authorization, X-Requested-With');
  headers.set('Access-Control-Max-Age', '86400');
  
  return new Response(null, {
    status: 200,
    headers: headers
  });
}

async function handleProxyRequest(request: NextRequest, method: string) {
  try {
    const { searchParams, pathname } = new URL(request.url);
    const path = searchParams.get('path') || '';
    
    // Determine target service URL
    const targetUrl = getTargetUrl(path);
    if (!targetUrl) {
      return NextResponse.json(
        { error: 'Invalid service path' },
        { status: 400 }
      );
    }
    
    // Validate authentication if required
    const authResult = await validateAuthentication(request, path);
    if (authResult.error) {
      return NextResponse.json(
        { error: authResult.error },
        { status: authResult.status || 401 }
      );
    }
    
    // Apply rate limiting
    const rateLimitResult = await checkRateLimit(request);
    if (rateLimitResult.error) {
      return NextResponse.json(
        { error: rateLimitResult.error },
        { 
          status: 429,
          headers: {
            'Retry-After': '60',
            'X-RateLimit-Limit': '100',
            'X-RateLimit-Remaining': '0',
            'X-RateLimit-Reset': Math.floor(Date.now() / 1000 + 60).toString()
          }
        }
      );
    }
    
    // Prepare headers for backend request
    const headers = new Headers();
    headers.set('Content-Type', request.headers.get('Content-Type') || 'application/json');
    headers.set('User-Agent', 'MEV-Dashboard-Proxy/1.0');
    headers.set('X-Forwarded-For', request.headers.get('X-Forwarded-For') || 'unknown');
    headers.set('X-Request-ID', generateRequestId());
    
    // Forward authentication header if present
    const authHeader = request.headers.get('Authorization');
    if (authHeader) {
      headers.set('Authorization', authHeader);
    }
    
    // Prepare request body for POST/PUT requests
    let body: string | undefined;
    if (method === 'POST' || method === 'PUT') {
      try {
        body = await request.text();
      } catch (error) {
        body = undefined;
      }
    }
    
    // Make request to backend service
    const response = await fetch(`${targetUrl}/${path}`, {
      method: method,
      headers: headers,
      body: body,
      signal: AbortSignal.timeout(30000) // 30 second timeout
    });
    
    // Prepare response headers
    const responseHeaders = new Headers();
    responseHeaders.set('Access-Control-Allow-Origin', '*');
    responseHeaders.set('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
    responseHeaders.set('Access-Control-Allow-Headers', 'Content-Type, Authorization, X-Requested-With');
    
    // Forward important headers from backend
    const headersToForward = [
      'Content-Type',
      'Cache-Control',
      'X-RateLimit-Limit',
      'X-RateLimit-Remaining',
      'X-RateLimit-Reset'
    ];
    
    headersToForward.forEach(headerName => {
      const headerValue = response.headers.get(headerName);
      if (headerValue) {
        responseHeaders.set(headerName, headerValue);
      }
    });
    
    // Add performance headers
    responseHeaders.set('X-Proxy-Timestamp', new Date().toISOString());
    responseHeaders.set('X-Response-Time', Date.now().toString());
    
    // Return response
    const responseData = await response.text();
    
    return new Response(responseData, {
      status: response.status,
      statusText: response.statusText,
      headers: responseHeaders
    });
    
  } catch (error) {
    console.error('Proxy request failed:', error);
    
    return NextResponse.json(
      { 
        error: 'Backend service unavailable',
        timestamp: new Date().toISOString(),
        requestId: generateRequestId()
      },
      { 
        status: 503,
        headers: {
          'Access-Control-Allow-Origin': '*',
          'Retry-After': '30'
        }
      }
    );
  }
}

function getTargetUrl(path: string): string | null {
  // Route to appropriate backend service based on path
  const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
  const strategyUrl = process.env.NEXT_PUBLIC_STRATEGY_SERVICE_URL || apiUrl;
  const replayUrl = process.env.NEXT_PUBLIC_REPLAY_SERVICE_URL || apiUrl;
  const forkUrl = process.env.NEXT_PUBLIC_FORK_SERVICE_URL || apiUrl;
  
  if (path.startsWith('strategy/')) {
    return strategyUrl;
  } else if (path.startsWith('replay/')) {
    return replayUrl;
  } else if (path.startsWith('fork/')) {
    return forkUrl;
  } else if (path.startsWith('health') || path.startsWith('api/')) {
    return apiUrl;
  }
  
  // Default to main API URL
  return apiUrl;
}

async function validateAuthentication(request: NextRequest, path: string): Promise<{ error?: string; status?: number }> {
  // Paths that don't require authentication
  const publicPaths = ['health', 'api/health', 'api/status'];
  
  if (publicPaths.some(publicPath => path.startsWith(publicPath))) {
    return {};
  }
  
  const authHeader = request.headers.get('Authorization');
  
  // For demonstration, we'll allow requests without auth but log them
  if (!authHeader) {
    console.warn('Request without authentication:', path);
    return {}; // Allow for now, but in production this should return an error
  }
  
  // Validate JWT token
  if (!authHeader.startsWith('Bearer ')) {
    return { error: 'Invalid authorization header format', status: 401 };
  }
  
  const token = authHeader.slice(7);
  
  // Basic token validation - in production, verify JWT signature
  if (token.length < 10) {
    return { error: 'Invalid token', status: 401 };
  }
  
  return {};
}

async function checkRateLimit(request: NextRequest): Promise<{ error?: string }> {
  // Simple rate limiting based on IP address
  // In production, use a proper rate limiting service like Upstash Redis
  
  const forwardedFor = request.headers.get('X-Forwarded-For');
  const ip = forwardedFor?.split(',')[0] || 'unknown';
  
  // For Edge Functions, we can't maintain state across requests
  // This would need to be implemented with external storage (Redis, KV store)
  // For now, we'll just log and allow
  
  console.log('Rate limit check for IP:', ip);
  
  return {};
}

function generateRequestId(): string {
  return `req_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
} 