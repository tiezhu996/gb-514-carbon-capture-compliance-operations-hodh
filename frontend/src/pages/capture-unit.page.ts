import { Component } from '@angular/core';
import { EntityPageComponent } from '../components/entity-page.component';
import { CaptureUnitStore } from '../stores/capture-unit.store';
import { ENTITY_CONFIGS } from '../types/status';

@Component({ selector: 'app-capture-unit-page', standalone: true, imports: [EntityPageComponent], template: `<app-entity-page [config]="config" [store]="store"/>` })
export class CaptureUnitPage { readonly config = ENTITY_CONFIGS[0]; constructor(readonly store: CaptureUnitStore) {} }
