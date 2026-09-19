
import { request } from './client';
import type { DomainRecord, RetirementCheck } from '../types/domain';

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
// Preflight the device-scoped verification (in-flight decisions, latest
// verified sample, device consistency) before showing the retire confirm.
export async function checkPermitRuleRetirement(id: number) {
  return request<RetirementCheck>(`/rules/${id}/retirement-check`);
}
// Retire is admin-only server-side. The backend re-runs the full verification
// inside a device-locked transaction, so a concurrent decision submit can only
// win against one of the two calls.
export async function retirePermitRule(id: number, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/rules/${id}/retire`, {
    method: 'POST', body: JSON.stringify({ expectedVersion, reason }),
  });
}
