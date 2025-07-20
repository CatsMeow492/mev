'use client';

import { useState, useCallback } from 'react';
import axios from 'axios';
import { MEVOpportunity, PerformanceMetrics, SystemStatus, HistoricalData } from '@/types/mev';

const api = axios.create({
  baseURL: '/api',
  timeout: 10000,
});

export const useApi = () => {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleRequest = useCallback(async <T>(request: () => Promise<T>): Promise<T | null> => {
    setLoading(true);
    setError(null);
    
    try {
      const result = await request();
      return result;
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'An error occurred';
      setError(errorMessage);
      console.error('API request failed:', err);
      return null;
    } finally {
      setLoading(false);
    }
  }, []);

  const getOpportunities = useCallback(async (
    limit = 50,
    strategy?: string,
    status?: string
  ): Promise<MEVOpportunity[] | null> => {
    return handleRequest(async () => {
      const params = new URLSearchParams();
      params.append('limit', limit.toString());
      if (strategy) params.append('strategy', strategy);
      if (status) params.append('status', status);
      
      const response = await api.get(`/opportunities?${params.toString()}`);
      return response.data;
    });
  }, [handleRequest]);

  const getOpportunity = useCallback(async (id: string): Promise<MEVOpportunity | null> => {
    return handleRequest(async () => {
      const response = await api.get(`/opportunities/${id}`);
      return response.data;
    });
  }, [handleRequest]);

  const getMetrics = useCallback(async (): Promise<PerformanceMetrics | null> => {
    return handleRequest(async () => {
      const response = await api.get('/metrics');
      return response.data;
    });
  }, [handleRequest]);

  const getSystemStatus = useCallback(async (): Promise<SystemStatus | null> => {
    return handleRequest(async () => {
      const response = await api.get('/status');
      return response.data;
    });
  }, [handleRequest]);

  const getHistoricalData = useCallback(async (
    period = '24h'
  ): Promise<HistoricalData[] | null> => {
    return handleRequest(async () => {
      const response = await api.get(`/historical?period=${period}`);
      return response.data;
    });
  }, [handleRequest]);

  const emergencyShutdown = useCallback(async (
    reason: string,
    override = false
  ): Promise<boolean> => {
    const result = await handleRequest(async () => {
      const response = await api.post('/shutdown', { reason, override });
      return response.data.success;
    });
    return result || false;
  }, [handleRequest]);

  const restartSystem = useCallback(async (): Promise<boolean> => {
    const result = await handleRequest(async () => {
      const response = await api.post('/restart');
      return response.data.success;
    });
    return result || false;
  }, [handleRequest]);

  const updateThresholds = useCallback(async (thresholds: Record<string, number>): Promise<boolean> => {
    const result = await handleRequest(async () => {
      const response = await api.post('/thresholds', thresholds);
      return response.data.success;
    });
    return result || false;
  }, [handleRequest]);

  return {
    loading,
    error,
    getOpportunities,
    getOpportunity,
    getMetrics,
    getSystemStatus,
    getHistoricalData,
    emergencyShutdown,
    restartSystem,
    updateThresholds,
  };
}; 