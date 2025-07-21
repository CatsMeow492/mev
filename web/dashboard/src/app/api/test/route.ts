import { NextResponse } from 'next/server';

export async function GET() {
  try {
    // Test direct connection to backend
    const response = await fetch('https://mev-strategy-dev.fly.dev/api/v1/status', {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    });

    const data = await response.json();

    return NextResponse.json({
      success: true,
      backend_status: 'connected',
      backend_data: data,
      frontend_url: 'dashboard-brand-beacon.vercel.app',
      backend_url: 'mev-strategy-dev.fly.dev',
      timestamp: new Date().toISOString()
    });

  } catch (error) {
    return NextResponse.json({
      success: false,
      backend_status: 'disconnected',
      error: error instanceof Error ? error.message : 'Unknown error',
      frontend_url: 'dashboard-brand-beacon.vercel.app',
      backend_url: 'mev-strategy-dev.fly.dev',
      timestamp: new Date().toISOString()
    }, { status: 500 });
  }
} 