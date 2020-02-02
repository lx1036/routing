import {Component, Input, OnInit} from '@angular/core';
import {AuthService} from '../shared/auth.service';
import {App} from '../shared/model/v1/app';
import {ClrDatagridStateInterface} from '@clr/angular';

@Component({
  selector: 'app-list-apps',
  template: `
      <clr-datagrid (clrDgRefresh)="refresh($event)" class="app-clr-datagrid">
          <clr-dg-column>
              <ng-container *clrDgHideableColumn="showState['name']">{{'TITLE.NAME' | translate}}</ng-container>
          </clr-dg-column>
          <clr-dg-column [clrDgField]="'description'">
              <ng-container *clrDgHideableColumn="showState['description']">{{'TITLE.DESCRIPTION' | translate}}</ng-container>
          </clr-dg-column>
          <clr-dg-column class="col-time">
              <ng-container *clrDgHideableColumn="showState['create_time']">{{'TITLE.CREATE_TIME' | translate}}</ng-container>
          </clr-dg-column>
          <clr-dg-column>
              <ng-container *clrDgHideableColumn="showState['create_user']">{{'TITLE.CREATE_USER' | translate}}</ng-container>
          </clr-dg-column>
          <clr-dg-column class="col-operate">
              <ng-container *clrDgHideableColumn="showState['action']">{{'TITLE.ACTION' | translate}}</ng-container>
          </clr-dg-column>

          <clr-dg-row *ngFor="let app of apps" [clrDgItem]="app">
              <clr-dg-cell><a href="javascript:void(0)" (click)="enterApp(app)">{{app.name}}</a></clr-dg-cell>
              <clr-dg-cell>{{app.description}}</clr-dg-cell>
              <clr-dg-cell class="col-time" style="font-family: monospace;">{{app.createTime | date:'yyyy-MM-dd HH:mm:ss'}}</clr-dg-cell>
              <clr-dg-cell>{{app.user}}</clr-dg-cell>
              <clr-dg-cell>
                  <button *ngIf="!app.starred" class="wayne-button text" (click)="starredApp(app)">{{'ACTION.COLLECT' | translate}}</button>
                  <button *ngIf="app.starred" class="wayne-button text" (click)="unstarredApp(app)">{{'ACTION.CANCEL_COLLECTED' | translate}}</button>
                  <button class="wayne-button text" (click)="enterApp(app)">{{'ACTION.DETAIL' | translate}}</button>
                  <button *ngIf="getMonitorUri()" class="wayne-button text" (click)="goToMonitor(app)">{{'ACTION.MONITOR' | translate}}</button>
                  <button *ngIf="authService.currentNamespacePermission.app.update || authService.currentUser.admin" class="wayne-button text" (click)="editApp(app)">{{'ACTION.EDIT' | translate}}</button>
                  <button *ngIf="authService.currentUser.admin" class="wayne-button text" (click)="deleteApp(app)">{{'ACTION.DELETE' | translate}}</button>
              </clr-dg-cell>
          </clr-dg-row>

          <clr-dg-footer>
              <app-paginate></app-paginate>
          </clr-dg-footer>
      </clr-datagrid>
  `
})

export class ListAppsComponent implements OnInit {
  showState: any;
  @Input() apps: App[];
  constructor(public authService: AuthService) {
  }

  ngOnInit() {
  }

  unstarredApp(app: App) {

  }

  deleteApp(app: App) {

  }

  refresh($event: ClrDatagridStateInterface) {

  }

  enterApp(app: App) {

  }

  goToMonitor(app: any) {

  }

  getMonitorUri() {

  }

  editApp(app: App) {

  }

  starredApp(app: App) {

  }
}
