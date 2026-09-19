import { Component } from '@angular/core';
import { EntityPageComponent } from '../components/entity-page.component';
import { PermitRuleStore } from '../stores/permit-rule.store';
import { ENTITY_CONFIGS } from '../types/status';

@Component({ selector: 'app-permit-rule-page', standalone: true, imports: [EntityPageComponent], template: `<app-entity-page [config]="config" [store]="store"/>` })
export class PermitRulePage { readonly config = ENTITY_CONFIGS[1]; constructor(readonly store: PermitRuleStore) {} }
