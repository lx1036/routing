import { BrowserModule } from '@angular/platform-browser';
import { NgModule } from '@angular/core';

import {RoutingModule} from './app-routing.module';
import { AppComponent } from './app.component';
import {PodTerminalModule} from "./portal/pod-terminal/pod-terminal.module";
import {AuthModule} from "./shared/auth-module/auth.module";
import {PortalModule} from "./portal/portal.module";
import {AdminModule} from "./admin/admin.module";
import {HttpClient, HttpClientModule} from "@angular/common/http";
import {TranslateLoader, TranslateModule} from "@ngx-translate/core";
import {TranslateHttpLoader} from "@ngx-translate/http-loader";
const packageJson = require('../../package.json');


export function HttpLoaderFactory(httpClient: HttpClient) {
  return new TranslateHttpLoader(httpClient, './assets/i18n/', '.json?v=' + packageJson.version);
}

@NgModule({
  declarations: [
    AppComponent
  ],
  imports: [
    BrowserModule,
    PodTerminalModule,
    AuthModule,
    PortalModule,
    AdminModule,
    RoutingModule,
    HttpClientModule,
    TranslateModule.forRoot({
      loader: {
        provide: TranslateLoader,
        useFactory: HttpLoaderFactory,
        deps: [HttpClient]
      }
    })
  ],
  providers: [],
  bootstrap: [AppComponent]
})
export class AppModule { }