import worldManger from "../plugins/worldmanager"

import jsonApi from "../plugins/jsonapi"


export default {
  subTableColumns(state) {
    return state.subTableColumns
  },
  isAuthenticated(state) {
    // console.log("check is authenticated: ", window.localStorage.getItem("token"))
    var x = JSON.parse(window.localStorage.getItem("user"));
    console.log("Auth check", x)
    if (!x || !x.exp || new Date(x.exp * 1000) < new Date()) {
      window.localStorage.removeItem("user")
      return false;
    }
    return !!window.localStorage.getItem("token")
  },
  systemActions(state) {
    return state.systemActions;
  },
  authToken(state) {
    return window.localStorage.getItem("token")
  },
  selectedAction(state) {
    return state.selectedAction;
  },
  selectedInstanceReferenceId(state) {
    return state.selectedInstanceReferenceId
  },
  user(state) {
    var user = JSON.parse(window.localStorage.getItem("user"));
    user = user || {};
    return user;
  },
  actions(state) {
    return state.actions;
  },
  selectedTable(state) {
    console.log("get selected table", state.selectedTable)
    return state.selectedTable;
  },
  finder(state) {
    return state.finder
  },
  selectedRow(state) {
    return state.selectedRow;
  },
  selectedTableColumns(state) {
    return state.selectedTableColumns;
  },
  selectedSubTable(state) {
    return state.selectedSubTable
  },
  showAddEdit(state) {
    return state.showAddEdit;
  },
  visibleWorlds(state) {
    let filtered = state.worlds.filter(function (w, r) {
      if (!state.selectedInstanceReferenceId) {
        // console.log("No selected item. Return top level tables")
        return w.is_top_level == 1 && w.is_hidden == 0;
      } else {
        // console.log("Selected item found. Return child tables")
        const model = jsonApi.modelFor(w.table_name);
        const attrs = model["attributes"];
        const keys = Object.keys(attrs);
        if (keys.indexOf(state.selectedTable + "_id") > -1) {
          return w.is_top_level == 0 && w.is_join_table == 0;
        }
        return false;
      }
    });
    console.log("filtered worlds: ", filtered)

    return filtered;
  },
  preferredLanguage(state) {
    console.log("get preferred language", state.language || navigator.language || navigator.userLanguage)
    return ( state.language || navigator.language || navigator.userLanguage).split('-')[0]
  },
  languages() {
    return [{"label": "Afrikaans", "id": "af"}, {"label": "Albanian", "id": "sq"}, {
      "label": "Amharic",
      "id": "am"
    }, {"label": "Arabic", "id": "ar"}, {"label": "Armenian", "id": "hy"}, {
      "label": "Assamese",
      "id": "as"
    }, {"label": "Azeri", "id": "az"}, {"label": "Basque", "id": "eu"}, {
      "label": "Belarusian",
      "id": "be"
    }, {"label": "Bengali", "id": "bn"}, {"label": "Bosnian", "id": "bs"}, {
      "label": "Bulgarian",
      "id": "bg"
    }, {"label": "Burmese", "id": "my"}, {"label": "Catalan", "id": "ca"}, {
      "label": "Chinese",
      "id": "zh"
    }, {"label": "Croatian", "id": "hr"}, {"label": "Czech", "id": "cs"}, {
      "label": "Danish",
      "id": "da"
    }, {"label": "Divehi", "id": "dv"}, {"label": "Dutch", "id": "nl"}, {
      "label": "English",
      "id": "en"
    }, {"label": "Estonian", "id": "et"}, {"label": "FYRO Macedonia", "id": "mk"}, {
      "label": "Faroese",
      "id": "fo"
    }, {"label": "Farsi", "id": "fa"}, {"label": "Finnish", "id": "fi"}, {
      "label": "French",
      "id": "fr"
    }, {"label": "Gaelic", "id": "gd"}, {"label": "Galician", "id": "gl"}, {
      "label": "Georgian",
      "id": "ka"
    }, {"label": "German", "id": "de"}, {"label": "Greek", "id": "el"}, {
      "label": "Guarani",
      "id": "gn"
    }, {"label": "Gujarati", "id": "gu"}, {"label": "Hebrew", "id": "he"}, {
      "label": "Hindi",
      "id": "hi"
    }, {"label": "Hungarian", "id": "hu"}, {"label": "Icelandic", "id": "is"}, {
      "label": "Indonesian",
      "id": "id"
    }, {"label": "Italian", "id": "it"}, {"label": "Japanese", "id": "ja"}, {
      "label": "Kannada",
      "id": "kn"
    }, {"label": "Kashmiri", "id": "ks"}, {"label": "Kazakh", "id": "kk"}, {
      "label": "Khmer",
      "id": "km"
    }, {"label": "Korean", "id": "ko"}, {"label": "Lao", "id": "lo"}, {
      "label": "Latin",
      "id": "la"
    }, {"label": "Latvian", "id": "lv"}, {"label": "Lithuanian", "id": "lt"}, {
      "label": "Malay",
      "id": "ms"
    }, {"label": "Malayalam", "id": "ml"}, {"label": "Maltese", "id": "mt"}, {
      "label": "Maori",
      "id": "mi"
    }, {"label": "Marathi", "id": "mr"}, {"label": "Mongolian", "id": "mn"}, {
      "label": "Nepali",
      "id": "ne"
    }, {"label": "Norwegian", "id": "nb"}, {"label": "Norwegian", "id": "nn"}, {
      "label": "Oriya",
      "id": "or"
    }, {"label": "Polish", "id": "pl"}, {"label": "Portuguese", "id": "pt"}, {
      "label": "Punjabi",
      "id": "pa"
    }, {"label": "Raeto-Romance", "id": "rm"}, {"label": "Romanian", "id": "ro"}, {
      "label": "Russian",
      "id": "ru"
    }, {"label": "Sanskrit", "id": "sa"}, {"label": "Serbian", "id": "sr"}, {
      "label": "Setsuana",
      "id": "tn"
    }, {"label": "Sindhi", "id": "sd"}, {"label": "Sinhala", "id": "si"}, {
      "label": "Slovak",
      "id": "sk"
    }, {"label": "Slovenian", "id": "sl"}, {"label": "Somali", "id": "so"}, {
      "label": "Sorbian",
      "id": "sb"
    }, {"label": "Spanish", "id": "es"}, {"label": "Swahili", "id": "sw"}, {
      "label": "Swedish",
      "id": "sv"
    }, {"label": "Tajik", "id": "tg"}, {"label": "Tamil", "id": "ta"}, {
      "label": "Tatar",
      "id": "tt"
    }, {"label": "Telugu", "id": "te"}, {"label": "Thai", "id": "th"}, {
      "label": "Tibetan",
      "id": "bo"
    }, {"label": "Tsonga", "id": "ts"}, {"label": "Turkish", "id": "tr"}, {
      "label": "Turkmen",
      "id": "tk"
    }, {"label": "Ukrainian", "id": "uk"}, {"label": "Urdu", "id": "ur"}, {
      "label": "Uzbek",
      "id": "uz"
    }, {"label": "Vietnamese", "id": "vi"}, {"label": "Welsh", "id": "cy"}, {
      "label": "Xhosa",
      "id": "xh"
    }, {"label": "Yiddish", "id": "yi"}, {"label": "Zulu", "id": "zu"}]
  }
}
