
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listComplianceDecision(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/decisions?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createComplianceDecision(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/decisions', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionComplianceDecision(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/decisions/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
