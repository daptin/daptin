const AppConfig = function () {

  const that = this;


  that.apiRoot = "http://localhost:6336";
  that.location = {
    protocol: "http:",
    host: "localhost:8080",
    hostname: "localhost",
  };
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
