(function(){
  loadjs = function () {
    function e(e, n) {
      var t, r, i, c = [],
          o = (e = e.push ? e : [e]).length,
          f = o;
      for (t = function (e, t) {
        t.length && c.push(e), --f || n(c)
      }; o--;) r = e[o], (i = s[r]) ? t(r, i) : (u[r] = u[r] || []).push(t)
    }

    function n(e, n) {
      if (e) {
        var t = u[e];
        if (s[e] = n, t)
          for (; t.length;) t[0](e, n), t.splice(0, 1)
      }
    }

    function t(e, n, r, i) {
      var o, s, u = document,
          f = r.async,
          a = (r.numRetries || 0) + 1,
          h = r.before || c;
      i = i || 0, /(^css!|\.css$)/.test(e) ? (o = !0, (s = u.createElement("link")).rel = "stylesheet", s.href = e.replace(/^css!/, "")) : ((s = u.createElement("script")).src = e, s.async = void 0 === f || f), s.onload = s.onerror = s.onbeforeload = function (c) {
        var u = c.type[0];
        if (o && "hideFocus" in s) try {
          s.sheet.cssText.length || (u = "e")
        } catch (e) {
          u = "e"
        }
        if ("e" == u && (i += 1) < a) return t(e, n, r, i);
        n(e, u, c.defaultPrevented)
      }, !1 !== h(e, s) && u.head.appendChild(s)
    }

    function r(e, n, r) {
      var i, c, o = (e = e.push ? e : [e]).length,
          s = o,
          u = [];
      for (i = function (e, t, r) {
        if ("e" == t && u.push(e), "b" == t) {
          if (!r) return;
          u.push(e)
        }
        --o || n(u)
      }, c = 0; c < s; c++) t(e[c], i, r)
    }

    function i(e, t, i) {
      var s, u;
      if (t && t.trim && (s = t), u = (s ? i : t) || {}, s) {
        if (s in o) throw "LoadJS";
        o[s] = !0
      }
      r(e, function (e) {
        e.length ? (u.error || c)(e) : (u.success || c)(), n(s, e)
      }, u)
    }

    var c = function () {
        },
        o = {},
        s = {},
        u = {};
    return i.ready = function (n, t) {
      return e(n, function (e) {
        e.length ? (t.error || c)(e) : (t.success || c)()
      }), i
    }, i.done = function (e) {
      n(e, [])
    }, i.reset = function () {
      o = {}, s = {}, u = {}
    }, i.isDefined = function (e) {
      return e in o
    }, i
  }();

  loadjs([
    'https://cdnjs.cloudflare.com/ajax/libs/jquery/2.2.0/jquery.min.js',
    'https://cdn.ckeditor.com/4.7.1/standard/ckeditor.js',
      // "//dashboard." + window.location.hostname +'/static/grapesjs/grapes.min.js',
      // "//dashboard." + window.location.hostname +'/static/grapesjs/css/grapes.min.css',
      "https://cdnjs.cloudflare.com/ajax/libs/grapesjs/0.12.17/css/grapes.min.css",
      "https://cdnjs.cloudflare.com/ajax/libs/grapesjs/0.12.17/grapes.min.js",
    'https://cdnjs.cloudflare.com/ajax/libs/underscore.js/1.8.3/underscore-min.js',
    'https://cdn.jsdelivr.net/gh/artf/grapesjs-navbar@master/dist/grapesjs-navbar.min.js',
    'https://cdn.jsdelivr.net/gh/artf/grapesjs-blocks-basic@master/dist/grapesjs-blocks-basic.min.js',
    'https://cdn.jsdelivr.net/gh/artf/grapesjs-plugin-forms@master/dist/grapesjs-plugin-forms.min.js',
    'https://cdn.jsdelivr.net/gh/artf/grapesjs-plugin-ckeditor@master/dist/grapesjs-plugin-ckeditor.min.js'

  ], 'goms', {
    success: function () {
      /* foo.js & bar.js loaded */
      console.log("loaded all js");


      var editor = grapesjs.init({
        container: 'body',
        allowscript: 1,
        plugins: [
          'gjs-plugin-ckeditor',
          'gjs-blocks-basic',
          'gjs-plugin-forms',
          'gjs-navbar'
        ],
        pluginOpts: {},
        storageManager: {
          type: 'remote',
          stepsBeforeSave: 5,
          autosave: true,
          urlStore: "//dashboard." + window.location.hostname + ":" + window.location.port + '/site/content/store?path=' + window.location.pathname,
          urlLoad: "//dashboard." + window.location.hostname + ":" + window.location.port + '/site/content/load?path=' + window.location.pathname,
          autoload: true,
          contentTypeJson: true,
          storeComponents: false,
          storeStyles: false,
          storeHtml: true,
          storeCss: true,
          params: {
            'path': window.location.pathname
          },
        }
      });
      editor.Panels.addButton
      ('options',
          [{
            id: 'save-db',
            className: 'fa fa-floppy-o',
            command: 'save-db',
            attributes: {title: 'Draft'}
          }]
      );
      editor.Commands.add
      ('save-db',
          {
            run: function (editor, sender) {
              sender && sender.set('active');
              editor.store();
            }
          }
      );

      var blockManager = editor.BlockManager;

      blockManager.add('my-first-block', {
        label: 'Simple block',
        content: '<div class="my-block">This is a simple block</div>',
      });

      blockManager.get('my-first-block').set({
        label: 'Updated simple block',
        attributes: {
          title: 'My title'
        }
      });
      blockManager.add('my-map-block', {
        label: 'Simple map block',
        content: {
          type: 'map',
          style: {
            height: '350px'
          },
          removable: false,
        }
      });

      blockManager.add('the-row-block', {
        label: '2 Columns',
        content: '<div class="row" data-gjs-droppable=".row-cell" data-gjs-custom-name="Row">' +
        '<div class="row-cell" data-gjs-draggable=".row"></div>' +
        '<div class="row-cell" data-gjs-draggable=".row"></div>' +
        '</div>',
      });
    },
    error: function (pathsNotFound) {
    }
  });
})();