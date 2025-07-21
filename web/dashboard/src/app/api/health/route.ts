import { NextRequest, NextResponse } from 'next/server';

export async function GET(request: NextRequest) {
  try {
    const timestamp = new Date().toISOString();
    const uptime = process.uptime();
    
    // Basic system health checks
    const memoryUsage = process.memoryUsage();
    const cpuUsage = process.cpuUsage();
    
    // Environment information
    const environment = process.env.NEXT_PUBLIC_APP_ENV || 'development';
    const version = process.env.VERCEL_GIT_COMMIT_SHA?.slice(0, 7) || 'dev';
    const region = process.env.VERCEL_REGION || 'local';
    
    // Check external service connectivity (if configured)
    const externalServices = await checkExternalServices();
    
    const healthData = {
      status: 'ok',
      timestamp,
      uptime: `${Math.floor(uptime)}s`,
      environment,
      version,
      region,
      system: {
        memory: {
          used: Math.round(memoryUsage.heapUsed / 1024 / 1024),
          total: Math.round(memoryUsage.heapTotal / 1024 / 1024),
          external: Math.round(memoryUsage.external / 1024 / 1024),
          unit: 'MB'
        },
        cpu: {
          user: cpuUsage.user,
          system: cpuUsage.system,
          unit: 'microseconds'
        }
      },
      services: externalServices,
      deployment: {
        vercelUrl: process.env.VERCEL_URL,
        customDomain: process.env.NEXT_PUBLIC_DOMAIN,
        buildId: process.env.VERCEL_GIT_COMMIT_SHA,
        deploymentUrl: process.env.VERCEL_DEPLOYMENT_URL,
      }
    };
    
    return NextResponse.json(healthData, {
      status: 200,
      headers: {
        'Cache-Control': 'no-cache, no-store, must-revalidate',
        'Pragma': 'no-cache',
        'Expires': '0',
      },
    });
    
  } catch (error) {
    console.error('Health check failed:', error);
    
    return NextResponse.json(
      {
        status: 'error',
        timestamp: new Date().toISOString(),
        error: error instanceof Error ? error.message : 'Unknown error',
      },
      {
        status: 503,
        headers: {
          'Cache-Control': 'no-cache, no-store, must-revalidate',
        },
      }
    );
  }
}

async function checkExternalServices() {
  const services = {
    api: { status: 'unknown', responseTime: null as number | null },
    websocket: { status: 'unknown', responseTime: null as number | null },
    strategy: { status: 'unknown', responseTime: null as number | null },
    replay: { status: 'unknown', responseTime: null as number | null },
    fork: { status: 'unknown', responseTime: null as number | null },
  };
  
  const apiUrl = process.env.NEXT_PUBLIC_API_URL;
  const strategyUrl = process.env.NEXT_PUBLIC_STRATEGY_SERVICE_URL;
  const replayUrl = process.env.NEXT_PUBLIC_REPLAY_SERVICE_URL;
  const forkUrl = process.env.NEXT_PUBLIC_FORK_SERVICE_URL;
  
  // Check main API service
  if (apiUrl) {
    try {
      const start = Date.now();
      const response = await fetch(`${apiUrl}/health`, { 
        method: 'GET',
        signal: AbortSignal.timeout(5000) // 5 second timeout
      });
      const responseTime = Date.now() - start;
      
      services.api = {
        status: response.ok ? 'healthy' : 'unhealthy',
        responseTime
      };
    } catch (error) {
      services.api = {
        status: 'unreachable',
        responseTime: null
      };
    }
  }
  
  // Check Strategy service
  if (strategyUrl) {
    try {
      const start = Date.now();
      const response = await fetch(`${strategyUrl}/health`, {
        method: 'GET',
        signal: AbortSignal.timeout(5000)
      });
      const responseTime = Date.now() - start;
      
      services.strategy = {
        status: response.ok ? 'healthy' : 'unhealthy',
        responseTime
      };
    } catch (error) {
      services.strategy = {
        status: 'unreachable',
        responseTime: null
      };
    }
  }
  
  // Check Replay service
  if (replayUrl) {
    try {
      const start = Date.now();
      const response = await fetch(`${replayUrl}/health`, {
        method: 'GET',
        signal: AbortSignal.timeout(5000)
      });
      const responseTime = Date.now() - start;
      
      services.replay = {
        status: response.ok ? 'healthy' : 'unhealthy',
        responseTime
      };
    } catch (error) {
      services.replay = {
        status: 'unreachable',
        responseTime: null
      };
    }
  }
  
  // Check Fork service
  if (forkUrl) {
    try {
      const start = Date.now();
      const response = await fetch(`${forkUrl}/health`, {
        method: 'GET',
        signal: AbortSignal.timeout(5000)
      });
      const responseTime = Date.now() - start;
      
      services.fork = {
        status: response.ok ? 'healthy' : 'unhealthy',
        responseTime
      };
    } catch (error) {
      services.fork = {
        status: 'unreachable',
        responseTime: null
      };
    }
  }
  
  return services;
}

// Handle HEAD requests for uptime monitoring
export async function HEAD(request: NextRequest) {
  return new Response(null, {
    status: 200,
    headers: {
      'Cache-Control': 'no-cache, no-store, must-revalidate',
    },
  });
}

// Handle OPTIONS for CORS preflight
export async function OPTIONS(request: NextRequest) {
  return new Response(null, {
    status: 200,
    headers: {
      'Allow': 'GET, HEAD, OPTIONS',
      'Access-Control-Allow-Origin': '*',
      'Access-Control-Allow-Methods': 'GET, HEAD, OPTIONS',
      'Access-Control-Allow-Headers': 'Content-Type',
    },
  });
} 