
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listCaptureUnit(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/units?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createCaptureUnit(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/units', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionCaptureUnit(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/units/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
