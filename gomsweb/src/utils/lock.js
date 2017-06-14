import {setSecret, unsetToken} from './auth'

import uuid from 'uuid'
import configManager from '../plugins/configmanager'


const getLock = (options) => {

  configManager.getAllConfig().then(function (configs) {
    return;
    const config = configs.auth0;
    const Auth0Lock = require('auth0-lock').default;
    return new Auth0Lock(config.AUTH0_CLIENT_ID, config.AUTH0_CLIENT_DOMAIN, options)

  }, function () {
    Notification.error("Failed to load config")
  });

};

const getBaseUrl = () => `${window.location.protocol}//${window.location.host}`;

const getOptions = (container) => {
  const secret = uuid.v4();
  setSecret(secret);
  return {
    container,
    closable: false,
    auth: {
      responseType: 'token',
      redirectUrl: `${getBaseUrl()}/`,
      params: {
        scope: 'openid profile email',
        state: secret
      }
    }
  }
};

export const show = (container) => getLock(getOptions(container)).show();
export const logout = () => {
  unsetToken()
};
