/**
 * Copyright (C) 2013 KO GmbH <copyright@kogmbh.com>
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

/*global window, define, require, document, dijit, dojo, runtime, ops*/

define("webodf/editor/Tools", [
    "dojo/ready",
    "dijit/MenuItem",
    "dijit/DropDownMenu",
    "dijit/form/Button",
    "dijit/form/DropDownButton",
    "dijit/Toolbar",
    "webodf/editor/widgets/paragraphAlignment",
    "webodf/editor/widgets/simpleStyles",
    "webodf/editor/widgets/undoRedoMenu",
    "webodf/editor/widgets/toolbarWidgets/currentStyle",
    "webodf/editor/widgets/annotation",
    "webodf/editor/widgets/editHyperlinks",
    "webodf/editor/widgets/imageInserter",
    "webodf/editor/widgets/paragraphStylesDialog",
    "webodf/editor/widgets/zoomSlider",
    "webodf/editor/widgets/aboutDialog",
    "webodf/editor/EditorSession"],
    function (ready, MenuItem, DropDownMenu, Button, DropDownButton, Toolbar, ParagraphAlignment, SimpleStyles, UndoRedoMenu, CurrentStyle, AnnotationControl, EditHyperlinks, ImageInserter, ParagraphStylesDialog, ZoomSlider, AboutDialog, EditorSession) {
        "use strict";

        return function Tools(toolbarElementId, args) {
            var tr = runtime.tr,
                onToolDone = args.onToolDone,
                loadOdtFile = args.loadOdtFile,
                saveOdtFile = args.saveOdtFile,
                saveAsOdtFile = args.saveAsOdtFile,
                downloadOdtFile = args.downloadOdtFile,
                close = args.close,
                toolbar,
                loadButton, saveButton, closeButton, aboutButton,
                saveAsButton, downloadButton,
                formatDropDownMenu, formatMenuButton,
                paragraphStylesMenuItem, paragraphStylesDialog,
                editorSession,
                aboutDialog,
                sessionSubscribers = [];

            function placeAndStartUpWidget(widget) {
                widget.placeAt(toolbar);
                widget.startup();
            }

            /**
             * Creates a tool and installs it, if the enabled flag is set to true.
             * Only supports tool classes whose constructor has a single argument which
             * is a callback to pass the created widget object to.
             * @param {!function(new:Object, function(!Object):undefined)} Tool  constructor method of the tool
             * @param {!boolean} enabled
             * @param {!Object|undefined=} config
             * @return {?Object}
             */
            function createTool(Tool, enabled, config) {
                var tool = null;

                if (enabled) {
                    if (config) {
                        tool = new Tool(config, placeAndStartUpWidget);
                    } else {
                        tool = new Tool(placeAndStartUpWidget);
                    }
                    sessionSubscribers.push(tool);
                    tool.onToolDone = onToolDone;
                    tool.setEditorSession(editorSession);
                }

                return tool;
            }

            function handleCursorMoved(cursor) {
                var disabled = cursor.getSelectionType() === ops.OdtCursor.RegionSelection;
                if (formatMenuButton) {
                    formatMenuButton.setAttribute('disabled', disabled);
                }
            }

            function setEditorSession(session) {
                if (editorSession) {
                    editorSession.unsubscribe(EditorSession.signalCursorMoved, handleCursorMoved);
                }

                editorSession = session;
                if (editorSession) {
                    editorSession.subscribe(EditorSession.signalCursorMoved, handleCursorMoved);
                }

                sessionSubscribers.forEach(function (subscriber) {
                    subscriber.setEditorSession(editorSession);
                });

                [saveButton, saveAsButton, downloadButton, closeButton, formatMenuButton].forEach(function (button) {
                    if (button) {
                        button.setAttribute('disabled', !editorSession);
                    }
                });
            }

            this.setEditorSession = setEditorSession;

            /**
             * @param {!function(!Error=)} callback, passing an error object in case of error
             * @return {undefined}
             */
            this.destroy = function (callback) {
                // TODO:
                // 1. We don't want to use `document`
                // 2. We would like to avoid deleting all widgets
                // under document.body because this might interfere with
                // other apps that use the editor not-in-an-iframe,
                // but dojo always puts its dialogs below the body,
                // so this works for now. Perhaps will be obsoleted
                // once we move to a better widget toolkit
                var widgets = dijit.findWidgets(document.body);
                dojo.forEach(widgets, function(w) {
                    w.destroyRecursive(false);
                });
                callback();
            };

            // init
            ready(function () {
                toolbar = new Toolbar({}, toolbarElementId);

                // About
                if (args.aboutEnabled) {
                    aboutButton = new Button({
                        label: tr('About WebODF Text Editor'),
                        showLabel: false,
                        iconClass: 'webodfeditor-dijitWebODFIcon'
                    });
                    aboutDialog = new AboutDialog(function (dialog) {
                        aboutButton.onClick = function () {
                            dialog.startup();
                            dialog.show();
                        };
                    });
                    aboutDialog.onToolDone = onToolDone;
                    aboutButton.placeAt(toolbar);
                }

                // Load
                if (loadOdtFile) {
                    loadButton = new Button({
                        label: tr('Open'),
                        showLabel: false,
                        iconClass: 'dijitIcon dijitIconFolderOpen',
                        onClick: function () {
                            loadOdtFile();
                        }
                    });
                    loadButton.placeAt(toolbar);
                }

                // Save
                if (saveOdtFile) {
                    saveButton = new Button({
                        label: tr('Save'),
                        showLabel: false,
                        disabled: true,
                        iconClass: 'dijitEditorIcon dijitEditorIconSave',
                        onClick: function () {
                            saveOdtFile();
                            onToolDone();
                        }
                    });
                    saveButton.placeAt(toolbar);
                }

                // SaveAs
                if (saveAsOdtFile) {
                    saveAsButton = new Button({
                        label: tr('Save as...'),
                        showLabel: false,
                        disabled: true,
                        iconClass: 'webodfeditor-dijitSaveAsIcon',
                        onClick: function () {
                            saveAsOdtFile();
                            onToolDone();
                        }
                    });
                    saveAsButton.placeAt(toolbar);
                }

                // Download
                if (downloadOdtFile) {
                    downloadButton = new Button({
                        label: tr('Download'),
                        showLabel: true,
                        disabled: true,
                        style: {
                            float: 'right'
                        },
                        onClick: function () {
                            downloadOdtFile();
                            onToolDone();
                        }
                    });
                    downloadButton.placeAt(toolbar);
                }

                // Format menu
                if (args.paragraphStyleEditingEnabled) {
                    formatDropDownMenu = new DropDownMenu({});
                    paragraphStylesMenuItem = new MenuItem({
                        label: tr("Paragraph...")
                    });
                    formatDropDownMenu.addChild(paragraphStylesMenuItem);

                    paragraphStylesDialog = new ParagraphStylesDialog(function (dialog) {
                        paragraphStylesMenuItem.onClick = function () {
                            if (editorSession) {
                                dialog.startup();
                                dialog.show();
                            }
                        };
                    });
                    sessionSubscribers.push(paragraphStylesDialog);
                    paragraphStylesDialog.onToolDone = onToolDone;

                    formatMenuButton = new DropDownButton({
                        dropDown: formatDropDownMenu,
                        disabled: true,
                        label: tr('Format'),
                        iconClass: "dijitIconEditTask"
                    });
                    formatMenuButton.placeAt(toolbar);
                }

                // Undo/Redo
                createTool(UndoRedoMenu, args.undoRedoEnabled);

                // Add annotation
                createTool(AnnotationControl, args.annotationsEnabled);

                // Simple Style Selector [B, I, U, S]
                createTool(SimpleStyles, args.directTextStylingEnabled);

                // Paragraph direct alignment buttons
                createTool(ParagraphAlignment, args.directParagraphStylingEnabled);

                // Paragraph Style Selector
                createTool(CurrentStyle, args.paragraphStyleSelectingEnabled);

                // Zoom Level Selector
                createTool(ZoomSlider, args.zoomingEnabled);

                // hyper links
                createTool(EditHyperlinks, args.hyperlinkEditingEnabled);

                // image insertion
                createTool(ImageInserter, args.imageInsertingEnabled);

                // close button
                if (close) {
                    closeButton = new Button({
                        label: tr('Close'),
                        showLabel: false,
                        disabled: true,
                        iconClass: 'dijitEditorIcon dijitEditorIconCancel',
                        style: {
                            float: 'right'
                        },
                        onClick: function () {
                            close();
                        }
                    });
                    closeButton.placeAt(toolbar);
                }

                // This is an internal hook for debugging/testing.
                // Yes, you discovered something interesting. But:
                // Do NOT rely on it, it will not be supported and can and will change in any version.
                // It is not officially documented for a reason. A real plugin system is only on the wishlist
                // so far, please file your suggestions/needs at the official WebODF issue system.
                // You have been warned.
                if (window.wodo_plugins) {
                    window.wodo_plugins.forEach(function (plugin) {
                        runtime.log("Creating plugin: "+plugin.id);
                        require([plugin.id], function (Plugin) {
                            runtime.log("Creating as tool now: "+plugin.id);
                            createTool(Plugin, true, plugin.config);
                        });
                    });

                }

                setEditorSession(editorSession);
            });
        };

    });
