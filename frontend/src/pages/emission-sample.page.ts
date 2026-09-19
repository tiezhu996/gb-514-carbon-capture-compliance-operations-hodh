import { Component } from '@angular/core';
import { EntityPageComponent } from '../components/entity-page.component';
import { EmissionSampleStore } from '../stores/emission-sample.store';
import { ENTITY_CONFIGS } from '../types/status';

@Component({ selector: 'app-emission-sample-page', standalone: true, imports: [EntityPageComponent], template: `<app-entity-page [config]="config" [store]="store"/>` })
export class EmissionSamplePage { readonly config = ENTITY_CONFIGS[2]; constructor(readonly store: EmissionSampleStore) {} }
