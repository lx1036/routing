import {App} from "./app";


export class Service {
  id: number;
  name: string;
  metaData: string;
  user: string;
  appId: number;
  metaDataObj: {};
  description: string;
  deleted: boolean;
  createTime: Date;
  app: App;
  order: number;
}

