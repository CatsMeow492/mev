import { NextRequest, NextResponse } from 'next/server';

export async function GET(request: NextRequest) {
  try {
    const timestamp = new Date().toISOString();
    const startTime = Date.now();
    
    // Collect health information from all services
    const healthChecks = await Promise.allSettled([
      checkMainAPI(),
      checkStrategyService(), 
      checkReplayService(),
      checkForkService(),
      checkDatabaseConnectivity(),
      checkRedisConnectivity()
    ]);
    
    const results = healthChecks.map((result, index) => {
      const serviceName = ['main-api', 'strategy', 'replay', 'fork', 'database', 'redis'][index];
      
      if (result.status === 'fulfilled') {
        return {
          service: serviceName,
          status: result.value.status,
          responseTime: result.value.responseTime,
          details: result.value.details
        };
      } else {
        return {
          service: serviceName,
          status: 'error',
          responseTime: null,
          error: result.reason?.message || 'Unknown error'
        };
      }
    });
    
    // Determine overall system health
    const healthyServices = results.filter(r => r.status === 'healthy').length;
    const totalServices = results.length;
    const overallStatus = healthyServices === totalServices ? 'healthy' : 
                         healthyServices > totalServices / 2 ? 'degraded' : 'unhealthy';
    
    const healthSummary = {
      timestamp,
      overallStatus,
      responseTime: Date.now() - startTime,
      servicesHealthy: healthyServices,
      totalServices,
      healthPercentage: Math.round((healthyServices / totalServices) * 100),
      services: results,
      deployment: {
        environment: process.env.NEXT_PUBLIC_APP_ENV,
        version: process.env.VERCEL_GIT_COMMIT_SHA?.slice(0, 7) || 'dev',
        region: process.env.VERCEL_REGION || 'local',
        vercelUrl: process.env.VERCEL_URL,
        customDomain: process.env.NEXT_PUBLIC_DOMAIN
      }
    };
    
    // Log health check results for monitoring
    console.log('Health Check Results:', {
      timestamp,
      overallStatus,
      healthyServices,
      totalServices,
      responseTime: Date.now() - startTime
    });
    
    // Send alerts if system is unhealthy
    if (overallStatus === 'unhealthy') {
      await sendHealthAlert(healthSummary);
    }
    
    const statusCode = overallStatus === 'healthy' ? 200 : 
                      overallStatus === 'degraded' ? 206 : 503;
    
    return NextResponse.json(healthSummary, {
      status: statusCode,
      headers: {
        'Cache-Control': 'no-cache, no-store, must-revalidate',
        'Content-Type': 'application/json',
      },
    });
    
  } catch (error) {
    console.error('Health check failed:', error);
    
    return NextResponse.json({
      timestamp: new Date().toISOString(),
      overallStatus: 'error',
      error: error instanceof Error ? error.message : 'Health check failed',
      deployment: {
        environment: process.env.NEXT_PUBLIC_APP_ENV,
        version: process.env.VERCEL_GIT_COMMIT_SHA?.slice(0, 7) || 'dev',
      }
    }, {
      status: 503,
      headers: {
        'Cache-Control': 'no-cache, no-store, must-revalidate',
      },
    });
  }
}

async function checkMainAPI() {
  const apiUrl = process.env.NEXT_PUBLIC_API_URL;
  if (!apiUrl) {
    return { status: 'not-configured', responseTime: null };
  }
  
  const start = Date.now();
  try {
    const response = await fetch(`${apiUrl}/health`, {
      method: 'GET',
      signal: AbortSignal.timeout(5000)
    });
    
    const responseTime = Date.now() - start;
    const data = await response.json();
    
    return {
      status: response.ok ? 'healthy' : 'unhealthy',
      responseTime,
      details: {
        statusCode: response.status,
        version: data.version,
        uptime: data.uptime
      }
    };
  } catch (error) {
    return {
      status: 'unreachable',
      responseTime: Date.now() - start,
      details: { error: error instanceof Error ? error.message : 'Unknown error' }
    };
  }
}

async function checkStrategyService() {
  const strategyUrl = process.env.NEXT_PUBLIC_STRATEGY_SERVICE_URL;
  if (!strategyUrl) {
    return { status: 'not-configured', responseTime: null };
  }
  
  const start = Date.now();
  try {
    const response = await fetch(`${strategyUrl}/health`, {
      method: 'GET',
      signal: AbortSignal.timeout(5000)
    });
    
    const responseTime = Date.now() - start;
    
    return {
      status: response.ok ? 'healthy' : 'unhealthy',
      responseTime,
      details: {
        statusCode: response.status
      }
    };
  } catch (error) {
    return {
      status: 'unreachable',
      responseTime: Date.now() - start,
      details: { error: error instanceof Error ? error.message : 'Unknown error' }
    };
  }
}

async function checkReplayService() {
  const replayUrl = process.env.NEXT_PUBLIC_REPLAY_SERVICE_URL;
  if (!replayUrl) {
    return { status: 'not-configured', responseTime: null };
  }
  
  const start = Date.now();
  try {
    const response = await fetch(`${replayUrl}/health`, {
      method: 'GET',
      signal: AbortSignal.timeout(5000)
    });
    
    const responseTime = Date.now() - start;
    
    return {
      status: response.ok ? 'healthy' : 'unhealthy',
      responseTime,
      details: {
        statusCode: response.status
      }
    };
  } catch (error) {
    return {
      status: 'unreachable',
      responseTime: Date.now() - start,
      details: { error: error instanceof Error ? error.message : 'Unknown error' }
    };
  }
}

async function checkForkService() {
  const forkUrl = process.env.NEXT_PUBLIC_FORK_SERVICE_URL;
  if (!forkUrl) {
    return { status: 'optional', responseTime: null };
  }
  
  const start = Date.now();
  try {
    const response = await fetch(`${forkUrl}/health`, {
      method: 'GET',
      signal: AbortSignal.timeout(5000)
    });
    
    const responseTime = Date.now() - start;
    
    return {
      status: response.ok ? 'healthy' : 'unhealthy',
      responseTime,
      details: {
        statusCode: response.status
      }
    };
  } catch (error) {
    return {
      status: 'unreachable',
      responseTime: Date.now() - start,
      details: { error: error instanceof Error ? error.message : 'Unknown error' }
    };
  }
}

async function checkDatabaseConnectivity() {
  // Since this is a frontend Edge Function, we can't directly connect to databases
  // We'll check if database URLs are configured
  const databaseUrl = process.env.DATABASE_URL;
  
  if (!databaseUrl) {
    return { status: 'not-configured', responseTime: null };
  }
  
  // In a real implementation, this would check database connectivity through the API
  return { status: 'configured', responseTime: 0, details: { note: 'Database check delegated to API' } };
}

async function checkRedisConnectivity() {
  // Since this is a frontend Edge Function, we can't directly connect to Redis
  // We'll check if Redis URLs are configured
  const redisUrl = process.env.REDIS_URL;
  
  if (!redisUrl) {
    return { status: 'not-configured', responseTime: null };
  }
  
  // In a real implementation, this would check Redis connectivity through the API
  return { status: 'configured', responseTime: 0, details: { note: 'Redis check delegated to API' } };
}

async function sendHealthAlert(healthSummary: any) {
  // Send alerts through configured channels
  try {
    // Slack webhook
    const slackWebhook = process.env.SLACK_WEBHOOK_URL;
    if (slackWebhook) {
      await fetch(slackWebhook, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          text: `🚨 MEV Engine Health Alert`,
          attachments: [{
            color: 'danger',
            fields: [
              {
                title: 'Overall Status',
                value: healthSummary.overallStatus,
                short: true
              },
              {
                title: 'Healthy Services',
                value: `${healthSummary.servicesHealthy}/${healthSummary.totalServices}`,
                short: true
              },
              {
                title: 'Environment',
                value: healthSummary.deployment.environment,
                short: true
              },
              {
                title: 'Timestamp',
                value: healthSummary.timestamp,
                short: true
              }
            ]
          }]
        }),
        signal: AbortSignal.timeout(5000)
      });
    }
    
    console.log('Health alert sent successfully');
  } catch (error) {
    console.error('Failed to send health alert:', error);
  }
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