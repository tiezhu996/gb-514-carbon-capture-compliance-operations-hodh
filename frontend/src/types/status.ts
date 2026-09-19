import type { EntityConfig } from './domain';

export type UnitState = 'standby' | 'running' | 'limited' | 'stopped';
export const ALL_UNIT_STATE: readonly UnitState[] = ['standby', 'running', 'limited', 'stopped'];
export type DecisionState = 'draft' | 'review' | 'accepted' | 'escalated';
export const ALL_DECISION_STATE: readonly DecisionState[] = ['draft', 'review', 'accepted', 'escalated'];

export const ENTITY_CONFIGS: readonly EntityConfig[] = [
  { key: 'captureUnit', path: 'units', label: '捕集装置', statuses: ['standby', 'running', 'limited', 'stopped'] as const },
  { key: 'permitRule', path: 'rules', label: '许可规则', statuses: ['draft', 'active', 'superseded', 'retired'] as const },
  { key: 'emissionSample', path: 'samples', label: '排放样本', statuses: ['collected', 'testing', 'verified', 'invalid'] as const },
  { key: 'complianceDecision', path: 'decisions', label: '合规决定', statuses: ['draft', 'review', 'accepted', 'escalated'] as const }
];
