/**
 * Copyright (C) 2014 KO GmbH <copyright@kogmbh.com>
 *
 * @licstart
 * This file is part of WebODF.
 *
 * WebODF is free software: you can redistribute it and/or modify it
 * under the terms of the GNU Affero General Public License (GNU AGPL)
 * as published by the Free Software Foundation, either version 3 of
 * the License, or (at your option) any later version.
 *
 * WebODF is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with WebODF.  If not, see <http://www.gnu.org/licenses/>.
 * @licend
 *
 * @source: http://www.webodf.org/
 * @source: https://github.com/kogmbh/WebODF/
 */

/*global document, window, runtime, FileReader, alert, Uint8Array, Blob, saveAs, Wodo*/

function createEditor() {
  "use strict";

  var editor = null,
    editorOptions,
    loadedFilename;

  /*jslint emptyblock: true*/
  /**
   * @return {undefined}
   */
  function startEditing() {
  }
  /*jslint emptyblock: false*/

  /**
   * extract document url from the url-fragment
   *
   * @return {?string}
   */
  function guessDocUrl() {
    var pos, docUrl = String(document.location);
    // If the URL has a fragment (#...), try to load the file it represents
    pos = docUrl.indexOf('#');
    if (pos !== -1) {
      docUrl = docUrl.substr(pos + 1);
      docUrl = "statics/welcome.odt";
    } else {
      docUrl = "statics/welcome.odt";
    }
    return docUrl || null;
  }

  function fileSelectHandler(evt) {
    var file, files, reader;
    files = (evt.target && evt.target.files) ||
      (evt.dataTransfer && evt.dataTransfer.files);
    function onLoadEnd() {
      if (reader.readyState === 2) {
        runtime.registerFile(file.name, reader.result);
        loadedFilename = file.name;
        editor.openDocumentFromUrl(loadedFilename, startEditing);
      }
    }
    if (files && files.length === 1) {
      if (!editor.isDocumentModified() ||
        window.confirm("There are unsaved changes to the file. Do you want to discard them?")) {
        editor.closeDocument(function() {
          file = files[0];
          reader = new FileReader();
          reader.onloadend = onLoadEnd;
          reader.readAsArrayBuffer(file);
        });
      }
    } else {
      alert("File could not be opened in this browser.");
    }
  }

  function enhanceRuntime() {
    var openedFiles = {},
      readFile = runtime.readFile;
    runtime.readFile = function (path, encoding, callback) {
      var array;
      if (openedFiles.hasOwnProperty(path)) {
        array = new Uint8Array(openedFiles[path]);
        callback(undefined, array);
      } else {
        return readFile(path, encoding, callback);
      }
    };
    runtime.registerFile = function (path, data) {
      openedFiles[path] = data;
    };
  }

  function createFileLoadForm() {
    var form = document.createElement("form"),
      input = document.createElement("input");

    function internalHandler(evt) {
      if (input.value !== "") {
        fileSelectHandler(evt);
      }
      // reset to "", so selecting the same file next time still trigger the change handler
      input.value = "";
    }
    form.appendChild(input);
    form.style.display = "none";
    input.id = "fileloader";
    input.setAttribute("type", "file");
    input.addEventListener("change", internalHandler, false);
    document.body.appendChild(form);
  }

  function load() {
    var form = document.getElementById("fileloader");
    if (!form) {
      enhanceRuntime();
      createFileLoadForm();
      form = document.getElementById("fileloader");
    }
    form.click();
  }

  function save() {
    function saveByteArrayLocally(err, data) {
      if (err) {
        alert(err);
        return;
      }
      // TODO: odfcontainer should have a property mimetype
      var mimetype = "application/vnd.oasis.opendocument.text",
        filename = loadedFilename || "doc.odt",
        blob = new Blob([data.buffer], {type: mimetype});
      saveAs(blob, filename);
      // TODO: hm, saveAs could fail or be cancelled
      editor.setDocumentModified(false);
    }

    editor.getDocumentAsByteArray(saveByteArrayLocally);
  }

  editorOptions = {
    loadCallback: load,
    saveCallback: save,
    allFeaturesEnabled: true
  };

  function onEditorCreated(err, e) {
    var docUrl = guessDocUrl();

    if (err) {
      // something failed unexpectedly
      alert(err);
      return;
    }

    editor = e;
    editor.setUserData({
      fullName: "WebODF-Curious",
      color:    "black"
    });

    window.addEventListener("beforeunload", function (e) {
      var confirmationMessage = "There are unsaved changes to the file.";

      if (editor.isDocumentModified()) {
        // Gecko + IE
        (e || window.event).returnValue = confirmationMessage;
        // Webkit, Safari, Chrome etc.
        return confirmationMessage;
      }
    });

    if (docUrl) {
      loadedFilename = docUrl;
      editor.openDocumentFromUrl(docUrl, startEditing);
    }
  }

  Wodo.createTextEditor('editorContainer', editorOptions, onEditorCreated);
}
