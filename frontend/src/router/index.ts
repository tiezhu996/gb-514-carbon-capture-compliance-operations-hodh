import { inject } from '@angular/core';
import type { CanActivateFn, Routes } from '@angular/router';
import { Router } from '@angular/router';
import { authState } from '../hooks/use-auth';
import { CaptureUnitPage } from '../pages/capture-unit.page';
import { PermitRulePage } from '../pages/permit-rule.page';
import { EmissionSamplePage } from '../pages/emission-sample.page';
import { ComplianceDecisionPage } from '../pages/compliance-decision.page';
import { AuditPage } from '../pages/audit.page';

const requireRole = (minimum: string): CanActivateFn => async () => {
  const router = inject(Router);
  try {
    await authState.authenticate();
    return authState.hasMinimumRole(minimum) || router.parseUrl('/units');
  } catch {
    return router.parseUrl('/units');
  }
};

export const routes: Routes = [
  { path: '', pathMatch: 'full', redirectTo: 'units' },
  { path: 'units', component: CaptureUnitPage, canActivate: [requireRole('viewer')] },
  { path: 'rules', component: PermitRulePage, canActivate: [requireRole('viewer')] },
  { path: 'samples', component: EmissionSamplePage, canActivate: [requireRole('viewer')] },
  { path: 'decisions', component: ComplianceDecisionPage, canActivate: [requireRole('viewer')] },
  { path: 'audit', component: AuditPage, canActivate: [requireRole('reviewer')] },
  { path: '**', redirectTo: 'units' },
];
