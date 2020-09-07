/**
 * Copyright (C) 2012-2013 KO GmbH <copyright@kogmbh.com>
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

/*global define, require, document, Image, FileReader, window, runtime, ops, gui */

define("webodf/editor/widgets/imageInserter", [
    "dijit/form/Button",
    "webodf/editor/EditorSession"],

    function (Button, EditorSession) {
        "use strict";

        var ImageInserter = function (callback) {
            var self = this,
                widget = {},
                insertImageButton,
                editorSession,
                fileLoader,
                textController,
                imageController;

            /**
             *
             * @param {!string} mimetype
             * @param {!string} content base64 encoded string
             * @param {!number} width
             * @param {!number} height
             */
            function insertImage(mimetype, content, width, height) {
                textController.removeCurrentSelection();
                imageController.insertImage(mimetype, content, width, height);
            }

            /**
             * @param {!string} content  as datauri
             * @param {!string} mimetype
             * @return {undefined}
             */
            function insertImageOnceLoaded(mimetype, content) {
                var hiddenImage = new Image();

                hiddenImage.style.position = "absolute";
                hiddenImage.style.left = "-99999px";
                document.body.appendChild(hiddenImage);
                hiddenImage.onload = function () {
                    // remove the data:image/jpg;base64, bit
                    content = content.substring(content.indexOf(",") + 1);
                    insertImage(mimetype, content, hiddenImage.width, hiddenImage.height);
                    // clean up
                    document.body.removeChild(hiddenImage);
                    self.onToolDone();
                };
                hiddenImage.src = content;
            }

            function fileSelectHandler(evt) {
                var file, files, reader;
                files = (evt.target && evt.target.files) || (evt.dataTransfer && evt.dataTransfer.files);
                if (files && files.length === 1) {
                    file = files[0];
                    reader = new FileReader();
                    reader.onloadend = function () {
                        if (reader.readyState === 2) {
                            insertImageOnceLoaded(file.type, reader.result);
                        } else {
                            runtime.log("Image could not be loaded");
                            self.onToolDone();
                        }
                    };
                    reader.readAsDataURL(file);
                }
            }

            function createFileLoader() {
                var form = document.createElement("form"),
                    input = document.createElement("input");
                form.appendChild(input);
                form.id = "imageForm";
                form.style.display = "none";
                input.id = "imageLoader";
                input.setAttribute("type", "file");
                input.setAttribute("accept", "image/*");
                input.addEventListener("change", fileSelectHandler, false);
                document.body.appendChild(form);
                return {input: input, form: form};
            }

            insertImageButton = new Button({
                label: runtime.tr("Insert Image"),
                disabled: true,
                showLabel: false,
                iconClass: "dijitEditorIcon dijitEditorIconInsertImage",
                onClick: function () {
                    if (!fileLoader) {
                        fileLoader = createFileLoader();
                    }
                    fileLoader.form.reset();
                    fileLoader.input.click();
                }
            });

            widget.children = [insertImageButton];
            widget.startup = function () {
                widget.children.forEach(function (element) {
                    element.startup();
                });
            };

            widget.placeAt = function (container) {
                widget.children.forEach(function (element) {
                    element.placeAt(container);
                });
                return widget;
            };

            function enableButtons(isEnabled) {
                widget.children.forEach(function (element) {
                    element.setAttribute('disabled', !isEnabled);
                });
            }
            function handleCursorMoved(cursor) {
                if (imageController.isEnabled()) {
                    var disabled = cursor.getSelectionType() === ops.OdtCursor.RegionSelection;
                    // LO/AOO pops up the picture/frame option dialog if image is selected when pressing the button
                    // Since we only support inline images, disable the button for now.
                    insertImageButton.setAttribute('disabled', disabled);
                }
            }

            this.setEditorSession = function (session) {
                if (editorSession) {
                    editorSession.unsubscribe(EditorSession.signalCursorMoved, handleCursorMoved);
                    imageController.unsubscribe(gui.ImageController.enabledChanged, enableButtons);
                }

                editorSession = session;
                if (editorSession) {
                    textController = editorSession.sessionController.getTextController();
                    imageController = editorSession.sessionController.getImageController();

                    editorSession.subscribe(EditorSession.signalCursorMoved, handleCursorMoved);
                    imageController.subscribe(gui.ImageController.enabledChanged, enableButtons);

                    enableButtons(imageController.isEnabled());
                } else {
                    enableButtons(false);
                }
            };

            /*jslint emptyblock: true*/
            this.onToolDone = function () {};
            /*jslint emptyblock: false*/

            callback(widget);
        };

        return ImageInserter;
    });
