
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listPermitRule(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/rules?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createPermitRule(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/rules', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionPermitRule(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/rules/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
