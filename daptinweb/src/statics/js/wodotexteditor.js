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

/*global window, document, alert, navigator, require, dojo, runtime, core, gui, ops, odf, WodoFromSource*/

/**
 * Namespace of the Wodo.TextEditor
 * @namespace
 * @name Wodo
 */
window.Wodo = window.Wodo || (function () {
    "use strict";

    function getInstallationPath() {
        /**
         * Sees to get the url of this script on top of the stack trace.
         * @param {!string|undefined} stack
         * @return {!string|undefined}
         */
        function getScriptUrlFromStack(stack) {
            var url, matches;

            if (typeof stack === "string" && stack) {
                /*jslint regexp: true*/
                matches = stack.match(/((?:http[s]?|file):\/\/[\/]?.+?\/[^:\)]*?)(?::\d+)(?::\d+)?/);
                /*jslint regexp: false*/
                url = matches && matches[1];
            }
            if (typeof url === "string" && url) {
                return url;
            }
            return undefined;
        }

        /**
         * Tries by various tricks to get the url of this script.
         * To be called if document.currentScript is not supported
         * @return {!string|undefined}
         */
        function getCurrentScriptElementSrcByTricks() {
            var scriptElements = document.getElementsByTagName("script");

            // if there is only one script, it must be this
            if (scriptElements.length === 1) {
                return scriptElements[0].src;
            }

            // otherwise get it from the stacktrace
            try {
                throw new Error();
            } catch (err) {
                return getScriptUrlFromStack(err.stack);
            }
        }

        var path = ".", scriptElementSrc,
            a, pathname, pos;

        if (document.currentScript && document.currentScript.src) {
            scriptElementSrc = document.currentScript.src;
        } else {
            scriptElementSrc = getCurrentScriptElementSrcByTricks();
        }

        if (scriptElementSrc) {
            a = document.createElement('a');
            a.href = scriptElementSrc;
            pathname = a.pathname;
            if (pathname.charAt(0) !== "/") {
                // Various versions of Internet Explorer seems to neglect the leading slash under some conditions
                // (not when watching it with the dev tools of course!). This was confirmed in IE10 + IE11
                pathname = "/" + pathname;
            }

            pos = pathname.lastIndexOf("/");
            if (pos !== -1) {
                path = pathname.substr(0, pos);
            }
        } else {
            alert("Could not estimate installation path of the Wodo.TextEditor.");
        }
        return path;
    }

    var /** @inner @const
            @type{!string} */
        installationPath = getInstallationPath(),
        /** @inner @type{!boolean} */
        isInitalized = false,
        /** @inner @type{!Array.<!function():undefined>} */
        pendingInstanceCreationCalls = [],
        /** @inner @type{!number} */
        instanceCounter = 0,
        // TODO: avatar image url needs base-url setting.
        // so far Wodo itself does not have a setup call,
        // but then the avatar is also not used yet here
        defaultUserData = {
            fullName: "",
            color:    "black",
            imageUrl: "avatar-joe.png"
        },
        /** @inner @const
            @type{!Array.<!string>} */
        userDataFieldNames = ["fullName", "color", "imageUrl"],
        /** @inner @const
            @type{!string} */
        memberId = "localuser",
        // constructors
        BorderContainer, ContentPane, FullWindowZoomHelper, EditorSession, Tools,
        /** @inner @const
            @type{!string} */
        MODUS_FULLEDITING = "fullediting",
        /** @inner @const
            @type{!string} */
        MODUS_REVIEW = "review",
        /** @inner @const
            @type{!string} */
        EVENT_UNKNOWNERROR = "unknownError",
        /** @inner @const
            @type {!string} */
        EVENT_DOCUMENTMODIFIEDCHANGED = "documentModifiedChanged",
        /** @inner @const
            @type {!string} */
        EVENT_METADATACHANGED = "metadataChanged";

    window.dojoConfig = (function () {
        var WebODFEditorDojoLocale = "C";

        if (navigator && navigator.language && navigator.language.match(/^(de)/)) {
            WebODFEditorDojoLocale = navigator.language.substr(0, 2);
        }

        return {
            locale: WebODFEditorDojoLocale,
            paths: {
                "webodf/editor": installationPath,
                "dijit":         installationPath + "/dijit",
                "dojox":         installationPath + "/dojox",
                "dojo":          installationPath + "/dojo",
                "resources":     installationPath + "/resources"
            }
        };
    }());

    /**
     * @return {undefined}
     */
    function initTextEditor() {
        require([
            "dijit/layout/BorderContainer",
            "dijit/layout/ContentPane",
            "webodf/editor/FullWindowZoomHelper",
            "webodf/editor/EditorSession",
            "webodf/editor/Tools",
            "webodf/editor/Translator"],
            function (BC, CP, FWZH, ES, T, Translator) {
                var locale = navigator.language || "en-US",
                    editorBase = dojo.config && dojo.config.paths && dojo.config.paths["webodf/editor"],
                    translationsDir = editorBase + '/translations',
                    t;

                BorderContainer = BC;
                ContentPane = CP;
                FullWindowZoomHelper = FWZH;
                EditorSession = ES;
                Tools = T;

                // TODO: locale cannot be set by the user, also different for different editors
                t = new Translator(translationsDir, locale, function (editorTranslator) {
                    runtime.setTranslator(editorTranslator.translate);
                    // Extend runtime with a convenient translation function
                    runtime.translateContent = function (node) {
                        var i,
                            element,
                            tag,
                            placeholder,
                            translatable = node.querySelectorAll("*[text-i18n]");

                        for (i = 0; i < translatable.length; i += 1) {
                            element = translatable[i];
                            tag = element.localName;
                            placeholder = element.getAttribute('text-i18n');
                            if (tag === "label"
                                    || tag === "span"
                                    || /h\d/i.test(tag)) {
                                element.textContent = runtime.tr(placeholder);
                            }
                        }
                    };

                    defaultUserData.fullName = runtime.tr("Unknown Author");

                    isInitalized = true;
                    pendingInstanceCreationCalls.forEach(function (create) { create(); });
                });

                // only done to make jslint see the var used
                return t;
            }
        );
    }

    /**
     * Creates a new record with userdata, and for all official fields
     * copies over the value from the original or, if not present there,
     * sets it to the default value.
     * @param {?Object.<!string,!string>|undefined} original, defaults to {}
     * @return {!Object.<!string,!string>}
     */
    function cloneUserData(original) {
        var result = {};

        if (!original) {
            original = {};
        }

        userDataFieldNames.forEach(function (fieldName) {
            result[fieldName] = original[fieldName] || defaultUserData[fieldName];
        });

        return result;
    }

    /**
     * @name TextEditor
     * @constructor
     * @param {!string} mainContainerElementId
     * @param {!Object.<!string,!*>} editorOptions
     */
    function TextEditor(mainContainerElementId, editorOptions) {
        instanceCounter = instanceCounter + 1;

        /**
        * Returns true if either all features are wanted and this one is not explicitely disabled
        * or if not all features are wanted by default and it is explicitely enabled
        * @param {?boolean|undefined} isFeatureEnabled explicit flag which enables a feature
        * @return {!boolean}
        */
        function isEnabled(isFeatureEnabled) {
            return editorOptions.allFeaturesEnabled ? (isFeatureEnabled !== false) : isFeatureEnabled;
        }

        var userData,
            //
            mainContainerElement = document.getElementById(mainContainerElementId),
            canvasElement,
            canvasContainerElement,
            toolbarElement,
            toolbarContainerElement, // needed because dijit toolbar overwrites direct classList
            editorElement,
            /** @inner @const
                @type{!string} */
            canvasElementId = "webodfeditor-canvas" + instanceCounter,
            /** @inner @const
                @type{!string} */
            canvasContainerElementId = "webodfeditor-canvascontainer" + instanceCounter,
            /** @inner @const
                @type{!string} */
            toolbarElementId = "webodfeditor-toolbar" + instanceCounter,
            /** @inner @const
                @type{!string} */
            editorElementId = "webodfeditor-editor" + instanceCounter,
            //
            fullWindowZoomHelper,
            //
            mainContainer,
            tools,
            odfCanvas,
            //
            editorSession,
            session,
            //
            loadOdtFile = editorOptions.loadCallback,
            saveOdtFile = editorOptions.saveCallback,
            saveAsOdtFile = editorOptions.saveAsCallback,
            downloadOdtFile = editorOptions.downloadCallback,
            close =       editorOptions.closeCallback,
            //
            reviewModeEnabled = (editorOptions.modus === MODUS_REVIEW),
            directTextStylingEnabled = isEnabled(editorOptions.directTextStylingEnabled),
            directParagraphStylingEnabled = isEnabled(editorOptions.directParagraphStylingEnabled),
            paragraphStyleSelectingEnabled = (!reviewModeEnabled) && isEnabled(editorOptions.paragraphStyleSelectingEnabled),
            paragraphStyleEditingEnabled =   (!reviewModeEnabled) && isEnabled(editorOptions.paragraphStyleEditingEnabled),
            imageEditingEnabled =            (!reviewModeEnabled) && isEnabled(editorOptions.imageEditingEnabled),
            hyperlinkEditingEnabled = isEnabled(editorOptions.hyperlinkEditingEnabled),
            annotationsEnabled = reviewModeEnabled || isEnabled(editorOptions.annotationsEnabled),
            undoRedoEnabled = isEnabled(editorOptions.undoRedoEnabled),
            zoomingEnabled = isEnabled(editorOptions.zoomingEnabled),
            //
            pendingMemberId,
            pendingEditorReadyCallback,
            //
            eventNotifier = new webodfcore.EventNotifier([
                EVENT_UNKNOWNERROR,
                EVENT_DOCUMENTMODIFIEDCHANGED,
                EVENT_METADATACHANGED
            ]);

        runtime.assert(Boolean(mainContainerElement), "No id of an existing element passed to Wodo.createTextEditor(): " + mainContainerElementId);

        /**
         * @param {!Object} changes
         * @return {undefined}
         */
        function relayMetadataSignal(changes) {
            eventNotifier.emit(EVENT_METADATACHANGED, changes);
        }

        /**
         * @param {!Object} changes
         * @return {undefined}
         */
        function relayModifiedSignal(modified) {
            eventNotifier.emit(EVENT_DOCUMENTMODIFIEDCHANGED, modified);
        }

        /**
         * @return {undefined}
         */
        function createSession() {
            var viewOptions = {
                    editInfoMarkersInitiallyVisible: false,
                    caretAvatarsInitiallyVisible: false,
                    caretBlinksOnRangeSelect: true
                };

            // create session around loaded document
            session = new ops.Session(odfCanvas);
            editorSession = new EditorSession(session, pendingMemberId, {
                viewOptions: viewOptions,
                directTextStylingEnabled: directTextStylingEnabled,
                directParagraphStylingEnabled: directParagraphStylingEnabled,
                paragraphStyleSelectingEnabled: paragraphStyleSelectingEnabled,
                paragraphStyleEditingEnabled: paragraphStyleEditingEnabled,
                imageEditingEnabled: imageEditingEnabled,
                hyperlinkEditingEnabled: hyperlinkEditingEnabled,
                annotationsEnabled: annotationsEnabled,
                zoomingEnabled: zoomingEnabled,
                reviewModeEnabled: reviewModeEnabled
            });
            if (undoRedoEnabled) {
                editorSession.sessionController.setUndoManager(new gui.TrivialUndoManager());
                editorSession.sessionController.getUndoManager().subscribe(gui.UndoManager.signalDocumentModifiedChanged, relayModifiedSignal);
            }

            // Relay any metadata changes to the Editor's consumer as an event
            editorSession.sessionController.getMetadataController().subscribe(gui.MetadataController.signalMetadataChanged, relayMetadataSignal);

            // and report back to caller
            pendingEditorReadyCallback();
            // reset
            pendingEditorReadyCallback = null;
            pendingMemberId = null;
        }

        /**
         * @return {undefined}
         */
        function startEditing() {
            runtime.assert(editorSession, "editorSession should exist here.");

            tools.setEditorSession(editorSession);
            editorSession.sessionController.insertLocalCursor();
            editorSession.sessionController.startEditing();
        }

        /**
         * @return {undefined}
         */
        function endEditing() {
            runtime.assert(editorSession, "editorSession should exist here.");

            tools.setEditorSession(undefined);
            editorSession.sessionController.endEditing();
            editorSession.sessionController.removeLocalCursor();
        }

        /**
         * Loads an ODT document into the editor.
         * @name TextEditor#openDocumentFromUrl
         * @function
         * @param {!string} docUrl url from which the ODT document can be loaded
         * @param {!function(!Error=):undefined} callback Called once the document has been opened, passes an error object in case of error
         * @return {undefined}
         */
        this.openDocumentFromUrl = function (docUrl, editorReadyCallback) {
            runtime.assert(docUrl, "document should be defined here.");
            runtime.assert(!pendingEditorReadyCallback, "pendingEditorReadyCallback should not exist here.");
            runtime.assert(!editorSession, "editorSession should not exist here.");
            runtime.assert(!session, "session should not exist here.");

            pendingMemberId = memberId;
            pendingEditorReadyCallback = function () {
                var op = new ops.OpAddMember();
                op.init({
                    memberid: memberId,
                    setProperties: userData
                });
                session.enqueue([op]);
                startEditing();
                if (editorReadyCallback) {
                    editorReadyCallback();
                }
            };

            odfCanvas.load(docUrl);
        };

        /**
         * Closes the document, and does cleanup.
         * @name TextEditor#closeDocument
         * @function
         * @param {!function(!Error=):undefined} callback  Called once the document has been closed, passes an error object in case of error
         * @return {undefined}
         */
        this.closeDocument = function (callback) {
            runtime.assert(session, "session should exist here.");

            endEditing();

            var op = new ops.OpRemoveMember();
            op.init({
                memberid: memberId
            });
            session.enqueue([op]);

            session.close(function (err) {
                if (err) {
                    callback(err);
                } else {
                    editorSession.sessionController.getMetadataController().unsubscribe(gui.MetadataController.signalMetadataChanged, relayMetadataSignal);
                    editorSession.destroy(function (err) {
                        if (err) {
                            callback(err);
                        } else {
                            editorSession = undefined;
                            session.destroy(function (err) {
                                if (err) {
                                    callback(err);
                                } else {
                                    session = undefined;
                                    callback();
                                }
                            });
                        }
                    });
                }
            });
        };

        /**
         * @name TextEditor#getDocumentAsByteArray
         * @function
         * @param {!function(err:?Error, file:!Uint8Array=):undefined} callback Called with the current document as ODT file as bytearray, passes an error object in case of error
         * @return {undefined}
         */
        this.getDocumentAsByteArray = function (callback) {
            var odfContainer = odfCanvas.odfContainer();

            if (odfContainer) {
                odfContainer.createByteArray(function (ba) {
                    callback(null, ba);
                }, function (errorString) {
                    callback(new Error(errorString || "Could not create bytearray from OdfContainer."));
                });
            } else {
                callback(new Error("No odfContainer set!"));
            }
        };

        /**
         * Sets the metadata fields from the given properties map.
         * Avoid setting certain fields since they are automatically set:
         *    dc:creator
         *    dc:date
         *    meta:editing-cycles
         *
         * The following properties are never used and will be removed for semantic
         * consistency from the document:
         *     meta:editing-duration
         *     meta:document-statistic
         *
         * Setting any of the above mentioned fields using this method will have no effect.
         *
         * @name TextEditor#setMetadata
         * @function
         * @param {?Object.<!string, !string>} setProperties A flat object that is a string->string map of field name -> value.
         * @param {?Array.<!string>} removedProperties An array of metadata field names (prefixed).
         * @return {undefined}
         */
        this.setMetadata = function (setProperties, removedProperties) {
            runtime.assert(editorSession, "editorSession should exist here.");

            editorSession.sessionController.getMetadataController().setMetadata(setProperties, removedProperties);
        };

        /**
         * Returns the value of the requested document metadata field.
         * @name TextEditor#getMetadata
         * @function
         * @param {!string} property A namespace-prefixed field name, for example
         * dc:creator
         * @return {?string}
         */
        this.getMetadata = function (property) {
            runtime.assert(editorSession, "editorSession should exist here.");

            return editorSession.sessionController.getMetadataController().getMetadata(property);
        };

        /**
         * Sets the data for the person that is editing the document.
         * The supported fields are:
         *     "fullName": the full name of the editing person
         *     "color": color to use for the user specific UI elements
         * @name TextEditor#setUserData
         * @function
         * @param {?Object.<!string,!string>|undefined} data
         * @return {undefined}
         */
        function setUserData(data) {
            userData = cloneUserData(data);
        }
        this.setUserData = setUserData;

        /**
         * Returns the data set for the person that is editing the document.
         * @name TextEditor#getUserData
         * @function
         * @return {!Object.<!string,!string>}
         */
        this.getUserData = function () {
            return cloneUserData(userData);
        };

        /**
         * Sets the current state of the document to be either the unmodified state
         * or a modified state.
         * If @p modified is @true and the current state was already a modified state,
         * this call has no effect and also does not remove the unmodified flag
         * from the state which has it set.
         *
         * @name TextEditor#setDocumentModified
         * @function
         * @param {!boolean} modified
         * @return {undefined}
         */
        this.setDocumentModified = function (modified) {
            runtime.assert(editorSession, "editorSession should exist here.");

            if (undoRedoEnabled) {
                editorSession.sessionController.getUndoManager().setDocumentModified(modified);
            }
        };

        /**
         * Returns if the current state of the document matches the unmodified state.
         * @name TextEditor#isDocumentModified
         * @function
         * @return {!boolean}
         */
        this.isDocumentModified = function () {
            runtime.assert(editorSession, "editorSession should exist here.");

            if (undoRedoEnabled) {
                return editorSession.sessionController.getUndoManager().isDocumentModified();
            }

            return false;
        };

        /**
         * @return {undefined}
         */
        function setFocusToOdfCanvas() {
            editorSession.sessionController.getEventManager().focus();
        }

        /**
         * @param {!function(!Error=):undefined} callback passes an error object in case of error
         * @return {undefined}
         */
        function destroyInternal(callback) {
            mainContainerElement.removeChild(editorElement);

            callback();
        }

        /**
         * Destructs the editor object completely.
         * @name TextEditor#destroy
         * @function
         * @param {!function(!Error=):undefined} callback Called once the destruction has been completed, passes an error object in case of error
         * @return {undefined}
         */
        this.destroy = function (callback) {
            var destroyCallbacks = [];

            // TODO: decide if some forced close should be done here instead of enforcing proper API usage
            runtime.assert(!session, "session should not exist here.");

            // TODO: investigate what else needs to be done
            mainContainer.destroyRecursive(true);

            destroyCallbacks = destroyCallbacks.concat([
                fullWindowZoomHelper.destroy,
                tools.destroy,
                odfCanvas.destroy,
                destroyInternal
            ]);

            webodfcore.Async.destroyAll(destroyCallbacks, callback);
        };

        // TODO:
        // this.openDocumentFromByteArray = openDocumentFromByteArray; see also https://github.com/kogmbh/WebODF/issues/375
        // setReadOnly: setReadOnly,

        /**
         * Registers a callback which should be called if the given event happens.
         * @name TextEditor#addEventListener
         * @function
         * @param {!string} eventId
         * @param {!Function} callback
         * @return {undefined}
         */
        this.addEventListener = eventNotifier.subscribe;
        /**
         * Unregisters a callback for the given event.
         * @name TextEditor#removeEventListener
         * @function
         * @param {!string} eventId
         * @param {!Function} callback
         * @return {undefined}
         */
        this.removeEventListener = eventNotifier.unsubscribe;


        /**
         * @return {undefined}
         */
        function init() {
            var editorPane,
                /** @inner @const
                    @type{!string} */
                documentns = document.documentElement.namespaceURI;

            /**
             * @param {!string} tagLocalName
             * @param {!string|undefined} id
             * @param {!string} className
             * @return {!Element}
             */
            function createElement(tagLocalName, id, className) {
                var element;
                element = document.createElementNS(documentns, tagLocalName);
                if (id) {
                    element.id = id;
                }
                element.classList.add(className);
                return element;
            }

            // create needed tree structure
            canvasElement = createElement('div', canvasElementId, "webodfeditor-canvas");
            canvasContainerElement = createElement('div', canvasContainerElementId, "webodfeditor-canvascontainer");
            toolbarElement = createElement('span', toolbarElementId, "webodfeditor-toolbar");
            toolbarContainerElement = createElement('span', undefined, "webodfeditor-toolbarcontainer");
            editorElement = createElement('div', editorElementId, "webodfeditor-editor");

            // put into tree
            canvasContainerElement.appendChild(canvasElement);
            toolbarContainerElement.appendChild(toolbarElement);
            editorElement.appendChild(toolbarContainerElement);
            editorElement.appendChild(canvasContainerElement);
            mainContainerElement.appendChild(editorElement);

            // style all elements with Dojo's claro.
            // Not nice to do this on body, but then there is no other way known
            // to style also all dialogs, which are attached directly to body
            document.body.classList.add("claro");

            // prevent browser translation service messing up internal address system
            // TODO: this should be done more centrally, but where exactly?
            canvasElement.setAttribute("translate", "no");
            canvasElement.classList.add("notranslate");

            // create widgets
            mainContainer = new BorderContainer({}, mainContainerElementId);

            editorPane = new ContentPane({
                region: 'center'
            }, editorElementId);
            mainContainer.addChild(editorPane);

            mainContainer.startup();

            tools = new Tools(toolbarElementId, {
                onToolDone: setFocusToOdfCanvas,
                loadOdtFile: loadOdtFile,
                saveOdtFile: saveOdtFile,
                saveAsOdtFile: saveAsOdtFile,
                downloadOdtFile: downloadOdtFile,
                close: close,
                directTextStylingEnabled: directTextStylingEnabled,
                directParagraphStylingEnabled: directParagraphStylingEnabled,
                paragraphStyleSelectingEnabled: paragraphStyleSelectingEnabled,
                paragraphStyleEditingEnabled: paragraphStyleEditingEnabled,
                imageInsertingEnabled: imageEditingEnabled,
                hyperlinkEditingEnabled: hyperlinkEditingEnabled,
                annotationsEnabled: annotationsEnabled,
                undoRedoEnabled: undoRedoEnabled,
                zoomingEnabled: zoomingEnabled,
                aboutEnabled: true
            });

            odfCanvas = new odf.OdfCanvas(canvasElement);
            odfCanvas.enableAnnotations(annotationsEnabled, true);

            odfCanvas.addListener("statereadychange", createSession);

            fullWindowZoomHelper = new FullWindowZoomHelper(toolbarContainerElement, canvasContainerElement);

            setUserData(editorOptions.userData);
        }

        init();
    }

    function loadDojoAndStuff(callback) {
        var head = document.getElementsByTagName("head")[0],
            frag = document.createDocumentFragment(),
            link,
            script;

        // append two link and two script elements to the header
        link = document.createElement("link");
        link.rel = "stylesheet";
        link.href = installationPath + "/app/resources/app.css";
        link.type = "text/css";
        link.async = false;
        frag.appendChild(link);
        link = document.createElement("link");
        link.rel = "stylesheet";
        link.href = installationPath + "/wodotexteditor.css";
        link.type = "text/css";
        link.async = false;
        frag.appendChild(link);
        script = document.createElement("script");
        script.src = installationPath + "/dojo-amalgamation.js";
        script["data-dojo-config"] = "async: true";
        script.charset = "utf-8";
        script.type = "text/javascript";
        script.async = false;
        frag.appendChild(script);
        script = document.createElement("script");
        script.src = installationPath + "/webodf.js";
        script.charset = "utf-8";
        script.type = "text/javascript";
        script.async = false;
        script.onload = callback;
        frag.appendChild(script);
        head.appendChild(frag);
    }

    /**
     * Creates a text editor object and returns it on success in the passed callback.
     * @name Wodo#createTextEditor
     * @function
     * @param {!string} editorContainerElementId id of the existing div element which will contain the editor (should be empty before)
     * @param editorOptions options to configure the features of the editor. All entries are optional
     * @param [editorOptions.modus=Wodo.MODUS_FULLEDITING] set the editing modus. Current options: Wodo.MODUS_FULLEDITING, Wodo.MODUS_REVIEW
     * @param [editorOptions.loadCallback] parameter-less callback method, adds a "Load" button to the toolbar which triggers this method
     * @param [editorOptions.saveCallback] parameter-less callback method, adds a "Save" button to the toolbar which triggers this method
     * @param [editorOptions.saveAsCallback] parameter-less callback method, adds a "Save as" button to the toolbar which triggers this method
     * @param [editorOptions.downloadCallback] parameter-less callback method, adds a "Download" button to the right of the toolbar which triggers this method
     * @param [editorOptions.closeCallback] parameter-less callback method, adds a "Save" button to the toolbar which triggers this method
     * @param [editorOptions.allFeaturesEnabled=false] if set to 'true', switches the default for all features from 'false' to 'true'
     * @param [editorOptions.directTextStylingEnabled=false] if set to 'true', enables the direct styling of text (e.g. bold/italic or font)
     * @param [editorOptions.directParagraphStylingEnabled=false] if set to 'true', enables the direct styling of paragraphs (e.g. indentation or alignement)
     * @param [editorOptions.paragraphStyleSelectingEnabled=false] if set to 'true', enables setting of defined paragraph styles to paragraphs
     * @param [editorOptions.paragraphStyleEditingEnabled=false] if set to 'true', enables the editing of defined paragraph styles
     * @param [editorOptions.imageEditingEnabled=false] if set to 'true', enables the insertion of images
     * @param [editorOptions.hyperlinkEditingEnabled=false] if set to 'true', enables the editing of hyperlinks
     * @param [editorOptions.annotationsEnabled=false] if set to 'true', enables the display and the editing of annotations
     * @param [editorOptions.undoRedoEnabled=false] if set to 'true', enables the Undo and Redo of editing actions
     * @param [editorOptions.zoomingEnabled=false] if set to 'true', enables the zooming tool
     * @param [editorOptions.userData] data about the user editing the document
     * @param [editorOptions.userData.fullName] full name of the user, used for annotations and in the metadata of the document
     * @param [editorOptions.userData.color="black"] color to use for any user related indicators like cursor or annotations
     * @param {!function(err:?Error, editor:!TextEditor=):undefined} onEditorCreated
     * @return {undefined}
     */
    function createTextEditor(editorContainerElementId, editorOptions, onEditorCreated) {
        /**
         * @return {undefined}
         */
        function create() {
            var editor = new TextEditor(editorContainerElementId, editorOptions);
            onEditorCreated(null, editor);
        }

        if (!isInitalized) {
            pendingInstanceCreationCalls.push(create);
            // first request?
            if (pendingInstanceCreationCalls.length === 1) {
                if (String(typeof WodoFromSource) === "undefined") {
                    loadDojoAndStuff(initTextEditor);
                } else {
                    initTextEditor();
                }
            }
        } else {
            create();
        }
    }


    /**
     * @lends Wodo#
     */
    return {
        createTextEditor: createTextEditor,
        // flags
        /** Id of full editing modus */
        MODUS_FULLEDITING: MODUS_FULLEDITING,
        /** Id of review modus */
        MODUS_REVIEW: MODUS_REVIEW,
        /** Id of event for an unkown error */
        EVENT_UNKNOWNERROR: EVENT_UNKNOWNERROR,
        /** Id of event if documentModified state changes */
        EVENT_DOCUMENTMODIFIEDCHANGED: EVENT_DOCUMENTMODIFIEDCHANGED,
        /** Id of event if metadata changes */
        EVENT_METADATACHANGED: EVENT_METADATACHANGED
    };
}());
