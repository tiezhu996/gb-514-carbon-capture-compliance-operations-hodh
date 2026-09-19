import { Component } from '@angular/core';
import { EntityPageComponent } from '../components/entity-page.component';
import { ComplianceDecisionStore } from '../stores/compliance-decision.store';
import { ENTITY_CONFIGS } from '../types/status';

@Component({ selector: 'app-compliance-decision-page', standalone: true, imports: [EntityPageComponent], template: `<app-entity-page [config]="config" [store]="store"/>` })
export class ComplianceDecisionPage { readonly config = ENTITY_CONFIGS[3]; constructor(readonly store: ComplianceDecisionStore) {} }
