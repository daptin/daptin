const AppConfig = function () {

  const that = this;


  that.apiRoot = window.location.protocol + "//" + window.location.host;

  that.location = {
    protocol: window.location.protocol,
    host: window.location.host,
    hostname: window.location.hostname,
  };

  if (that.location.hostname === "site.daptin.com") {
    that.apiRoot = that.location.protocol + "//api.daptin.com:6336"
  }


  const that1 = this;

  that1.data = {};

  that.localStorage = {
    getItem: function (key) {
      return that1.data[key]
    },
    setItem: function (key, item) {
      that1.data[key] = item;
    }
  };

  return that;
};


const appconfig = new AppConfig();


export default appconfig
