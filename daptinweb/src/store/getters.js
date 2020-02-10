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
    let lang = localStorage.getItem("LANGUAGE");
    if (!lang) {
      lang = navigator.language || navigator.userLanguage;
    }
    localStorage.setItem("LANGUAGE", lang)
    console.log("get preferred language", lang)
    return (lang).split('-')[0]
  },
  languages() {
    return [{"label": "Abkhazian ", "id": "ab"},
      {"label": "Afar ", "id": "aa"},
      {"label": "Afrikaans ", "id": "af"},
      {"label": "Akan ", "id": "ak"},
      {"label": "Albanian ", "id": "sq"},
      {"label": "Amharic ", "id": "am"},
      {"label": "Arabic ", "id": "ar"},
      {"label": "Aragonese ", "id": "an"},
      {"label": "Armenian ", "id": "hy"},
      {"label": "Assamese ", "id": "as"},
      {"label": "Avaric ", "id": "av"},
      {"label": "Avestan ", "id": "ae"},
      {"label": "Aymara ", "id": "ay"},
      {"label": "Azerbaijani ", "id": "az"},
      {"label": "Bambara ", "id": "bm"},
      {"label": "Bashkir ", "id": "ba"},
      {"label": "Basque ", "id": "eu"},
      {"label": "Belarusian ", "id": "be"},
      {"label": "Bengali (Bangla) ", "id": "bn"},
      {"label": "Bihari ", "id": "bh"},
      {"label": "Bislama ", "id": "bi"},
      {"label": "Bosnian ", "id": "bs"},
      {"label": "Breton ", "id": "br"},
      {"label": "Bulgarian ", "id": "bg"},
      {"label": "Burmese ", "id": "my"},
      {"label": "Catalan ", "id": "ca"},
      {"label": "Chamorro ", "id": "ch"},
      {"label": "Chechen ", "id": "ce"},
      {"label": "Chichewa, Chewa, Nyanja ", "id": "ny"},
      {"label": "Chinese ", "id": "zh"},
      {"label": "Chinese (Simplified) ", "id": "zh-Hans"},
      {"label": "Chinese (Traditional) ", "id": "zh-Hant"},
      {"label": "Chuvash ", "id": "cv"},
      {"label": "Cornish ", "id": "kw"},
      {"label": "Corsican ", "id": "co"},
      {"label": "Cree ", "id": "cr"},
      {"label": "Croatian ", "id": "hr"},
      {"label": "Czech ", "id": "cs"},
      {"label": "Danish ", "id": "da"},
      {"label": "Divehi, Dhivehi, Maldivian ", "id": "dv"},
      {"label": "Dutch ", "id": "nl"},
      {"label": "Dzongkha ", "id": "dz"},
      {"label": "English ", "id": "en"},
      {"label": "Esperanto ", "id": "eo"},
      {"label": "Estonian ", "id": "et"},
      {"label": "Ewe ", "id": "ee"},
      {"label": "Faroese ", "id": "fo"},
      {"label": "Fijian ", "id": "fj"},
      {"label": "Finnish ", "id": "fi"},
      {"label": "French ", "id": "fr"},
      {"label": "Fula, Fulah, Pulaar, Pular ", "id": "ff"},
      {"label": "Galician ", "id": "gl"},
      {"label": "Gaelic (Scottish) ", "id": "gd"},
      {"label": "Gaelic (Manx) ", "id": "gv"},
      {"label": "Georgian ", "id": "ka"},
      {"label": "German ", "id": "de"},
      {"label": "Greek ", "id": "el"},
      {"label": "Greenlandic ", "id": "kl"},
      {"label": "Guarani ", "id": "gn"},
      {"label": "Gujarati ", "id": "gu"},
      {"label": "Haitian Creole ", "id": "ht"},
      {"label": "Hausa ", "id": "ha"},
      {"label": "Hebrew ", "id": "he"},
      {"label": "Herero ", "id": "hz"},
      {"label": "Hindi ", "id": "hi"},
      {"label": "Hiri Motu ", "id": "ho"},
      {"label": "Hungarian ", "id": "hu"},
      {"label": "Icelandic ", "id": "is"},
      {"label": "Ido ", "id": "io"},
      {"label": "Igbo ", "id": "ig"},
      {"label": "Indonesian ", "id": "id, in"},
      {"label": "Interlingua ", "id": "ia"},
      {"label": "Interlingue ", "id": "ie"},
      {"label": "Inuktitut ", "id": "iu"},
      {"label": "Inupiak ", "id": "ik"},
      {"label": "Irish ", "id": "ga"},
      {"label": "Italian ", "id": "it"},
      {"label": "Japanese ", "id": "ja"},
      {"label": "Javanese ", "id": "jv"},
      {"label": "Kalaallisut, Greenlandic ", "id": "kl"},
      {"label": "Kannada ", "id": "kn"},
      {"label": "Kanuri ", "id": "kr"},
      {"label": "Kashmiri ", "id": "ks"},
      {"label": "Kazakh ", "id": "kk"},
      {"label": "Khmer ", "id": "km"},
      {"label": "Kikuyu ", "id": "ki"},
      {"label": "Kinyarwanda (Rwanda) ", "id": "rw"},
      {"label": "Kirundi ", "id": "rn"},
      {"label": "Kyrgyz ", "id": "ky"},
      {"label": "Komi ", "id": "kv"},
      {"label": "Kongo ", "id": "kg"},
      {"label": "Korean ", "id": "ko"},
      {"label": "Kurdish ", "id": "ku"},
      {"label": "Kwanyama ", "id": "kj"},
      {"label": "Lao ", "id": "lo"},
      {"label": "Latin ", "id": "la"},
      {"label": "Latvian (Lettish) ", "id": "lv"},
      {"label": "Limburgish ( Limburger) ", "id": "li"},
      {"label": "Lingala ", "id": "ln"},
      {"label": "Lithuanian ", "id": "lt"},
      {"label": "Luga-Katanga ", "id": "lu"},
      {"label": "Luganda, Ganda ", "id": "lg"},
      {"label": "Luxembourgish ", "id": "lb"},
      {"label": "Manx ", "id": "gv"},
      {"label": "Macedonian ", "id": "mk"},
      {"label": "Malagasy ", "id": "mg"},
      {"label": "Malay ", "id": "ms"},
      {"label": "Malayalam ", "id": "ml"},
      {"label": "Maltese ", "id": "mt"},
      {"label": "Maori ", "id": "mi"},
      {"label": "Marathi ", "id": "mr"},
      {"label": "Marshallese ", "id": "mh"},
      {"label": "Moldavian ", "id": "mo"},
      {"label": "Mongolian ", "id": "mn"},
      {"label": "Nauru ", "id": "na"},
      {"label": "Navajo ", "id": "nv"},
      {"label": "Ndonga ", "id": "ng"},
      {"label": "Northern Ndebele ", "id": "nd"},
      {"label": "Nepali ", "id": "ne"},
      {"label": "Norwegian ", "id": "no"},
      {"label": "Norwegian bokmål ", "id": "nb"},
      {"label": "Norwegian nynorsk ", "id": "nn"},
      {"label": "Nuosu ", "id": "ii"},
      {"label": "Occitan ", "id": "oc"},
      {"label": "Ojibwe ", "id": "oj"},
      {"label": "Old Church Slavonic, Old Bulgarian ", "id": "cu"},
      {"label": "Oriya ", "id": "or"},
      {"label": "Oromo (Afaan Oromo) ", "id": "om"},
      {"label": "Ossetian ", "id": "os"},
      {"label": "Pāli ", "id": "pi"},
      {"label": "Pashto, Pushto ", "id": "ps"},
      {"label": "Persian (Farsi) ", "id": "fa"},
      {"label": "Polish ", "id": "pl"},
      {"label": "Portuguese ", "id": "pt"},
      {"label": "Punjabi (Eastern) ", "id": "pa"},
      {"label": "Quechua ", "id": "qu"},
      {"label": "Romansh ", "id": "rm"},
      {"label": "Romanian ", "id": "ro"},
      {"label": "Russian ", "id": "ru"},
      {"label": "Sami ", "id": "se"},
      {"label": "Samoan ", "id": "sm"},
      {"label": "Sango ", "id": "sg"},
      {"label": "Sanskrit ", "id": "sa"},
      {"label": "Serbian ", "id": "sr"},
      {"label": "Serbo-Croatian ", "id": "sh"},
      {"label": "Sesotho ", "id": "st"},
      {"label": "Setswana ", "id": "tn"},
      {"label": "Shona ", "id": "sn"},
      {"label": "Sichuan Yi ", "id": "ii"},
      {"label": "Sindhi ", "id": "sd"},
      {"label": "Sinhalese ", "id": "si"},
      {"label": "Siswati ", "id": "ss"},
      {"label": "Slovak ", "id": "sk"},
      {"label": "Slovenian ", "id": "sl"},
      {"label": "Somali ", "id": "so"},
      {"label": "Southern Ndebele ", "id": "nr"},
      {"label": "Spanish ", "id": "es"},
      {"label": "Sundanese ", "id": "su"},
      {"label": "Swahili (Kiswahili) ", "id": "sw"},
      {"label": "Swati ", "id": "ss"},
      {"label": "Swedish ", "id": "sv"},
      {"label": "Tagalog ", "id": "tl"},
      {"label": "Tahitian ", "id": "ty"},
      {"label": "Tajik ", "id": "tg"},
      {"label": "Tamil ", "id": "ta"},
      {"label": "Tatar ", "id": "tt"},
      {"label": "Telugu ", "id": "te"},
      {"label": "Thai ", "id": "th"},
      {"label": "Tibetan ", "id": "bo"},
      {"label": "Tigrinya ", "id": "ti"},
      {"label": "Tonga ", "id": "to"},
      {"label": "Tsonga ", "id": "ts"},
      {"label": "Turkish ", "id": "tr"},
      {"label": "Turkmen ", "id": "tk"},
      {"label": "Twi ", "id": "tw"},
      {"label": "Uyghur ", "id": "ug"},
      {"label": "Ukrainian ", "id": "uk"},
      {"label": "Urdu ", "id": "ur"},
      {"label": "Uzbek ", "id": "uz"},
      {"label": "Venda ", "id": "ve"},
      {"label": "Vietnamese ", "id": "vi"},
      {"label": "Volapük ", "id": "vo"},
      {"label": "Wallon ", "id": "wa"},
      {"label": "Welsh ", "id": "cy"},
      {"label": "Wolof ", "id": "wo"},
      {"label": "Western Frisian ", "id": "fy"},
      {"label": "Xhosa ", "id": "xh"},
      {"label": "Yiddish ", "id": "yi, ji"},
      {"label": "Yoruba ", "id": "yo"},
      {"label": "Zhuang, Chuang ", "id": "za"},
      {"label": "Zulu ", "id": "zu"},
    ]
  }
}
