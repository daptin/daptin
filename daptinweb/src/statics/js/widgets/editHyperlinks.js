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

/*global define, require, document, odf, runtime, core, gui */

define("webodf/editor/widgets/editHyperlinks", [
    "webodf/editor/EditorSession",
    "webodf/editor/widgets/dialogWidgets/editHyperlinkPane",
    "dijit/form/Button",
    "dijit/form/DropDownButton",
    "dijit/TooltipDialog"],

    function (EditorSession, EditHyperlinkPane, Button, DropDownButton, TooltipDialog) {
        "use strict";

        runtime.loadClass("odf.OdfUtils");
        runtime.loadClass("odf.TextSerializer");
        runtime.loadClass("core.EventSubscriptions");

        var EditHyperlinks = function (callback) {
            var self = this,
                widget = {},
                editorSession,
                hyperlinkController,
                linkEditorContent,
                editHyperlinkButton,
                removeHyperlinkButton,
                odfUtils = odf.OdfUtils,
                textSerializer = new odf.TextSerializer(),
                eventSubscriptions = new webodfcore.EventSubscriptions(),
                dialog;

            function updateLinkEditorContent() {
                var selection = editorSession.getSelectedRange(),
                    linksInSelection = editorSession.getSelectedHyperlinks(),
                    linkTarget = linksInSelection[0] ? odfUtils.getHyperlinkTarget(linksInSelection[0]) : "http://";

                if (selection && selection.collapsed && linksInSelection.length === 1) {
                    // Selection is collapsed within a single hyperlink. Assume user is modifying the hyperlink
                    linkEditorContent.set({
                        linkDisplayText: textSerializer.writeToString(linksInSelection[0]),
                        linkUrl: linkTarget,
                        isReadOnlyText: true
                    });
                } else if (selection && !selection.collapsed) {
                    // User has selected part of a hyperlink or a block of text. Assume user is attempting to modify the
                    // existing hyperlink, or wants to convert the selection into a hyperlink
                    linkEditorContent.set({
                        // TODO Improve performance by rewriting to not clone the range contents
                        linkDisplayText: textSerializer.writeToString(selection.cloneContents()),
                        linkUrl: linkTarget,
                        isReadOnlyText: true
                    });
                } else {
                    // Selection is collapsed and is not in an existing hyperlink
                    linkEditorContent.set({
                        linkDisplayText: "",
                        linkUrl: linkTarget,
                        isReadOnlyText: false
                    });
                }
            }

            function updateHyperlinkButtons() {
                var controllerEnabled = hyperlinkController && hyperlinkController.isEnabled(),
                    linksInSelection;

                // Enable to disable all widgets initially
                widget.children.forEach(function (element) {
                    element.set('disabled', controllerEnabled !== true, false);
                });
                if (controllerEnabled) {
                    // Specifically enable the remove hyperlink button only if there are links in the current selection
                    linksInSelection = editorSession.getSelectedHyperlinks();
                    removeHyperlinkButton.set('disabled', linksInSelection.length === 0, false);
                }
            }

            function updateSelectedLink(hyperlinkData) {
                var selection = editorSession.getSelectedRange(),
                    selectionController = editorSession.sessionController.getSelectionController(),
                    selectedLinkRange,
                    linksInSelection = editorSession.getSelectedHyperlinks();

                if (hyperlinkData.isReadOnlyText === "true") {
                    if (selection && selection.collapsed && linksInSelection.length === 1) {
                        // Editing the single link the cursor is currently within
                        selectedLinkRange = selection.cloneRange();
                        selectedLinkRange.selectNode(linksInSelection[0]);
                        selectionController.selectRange(selectedLinkRange, true);
                    }
                    hyperlinkController.removeHyperlinks();
                    hyperlinkController.addHyperlink(hyperlinkData.linkUrl);
                } else {
                    hyperlinkController.addHyperlink(hyperlinkData.linkUrl, hyperlinkData.linkDisplayText);
                    linksInSelection = editorSession.getSelectedHyperlinks();
                    selectedLinkRange = selection.cloneRange();
                    selectedLinkRange.selectNode(linksInSelection[0]);
                    selectionController.selectRange(selectedLinkRange, true);
                }
            }

            this.setEditorSession = function (session) {
                eventSubscriptions.unsubscribeAll();
                hyperlinkController = undefined;
                editorSession = session;
                if (editorSession) {
                    hyperlinkController = editorSession.sessionController.getHyperlinkController();
                    eventSubscriptions.addFrameSubscription(editorSession, EditorSession.signalCursorMoved, updateHyperlinkButtons);
                    eventSubscriptions.addFrameSubscription(editorSession, EditorSession.signalParagraphChanged, updateHyperlinkButtons);
                    eventSubscriptions.addFrameSubscription(editorSession, EditorSession.signalParagraphStyleModified, updateHyperlinkButtons);
                    eventSubscriptions.addSubscription(hyperlinkController, gui.HyperlinkController.enabledChanged, updateHyperlinkButtons);
                }
                updateHyperlinkButtons();
            };

            /*jslint emptyblock: true*/
            this.onToolDone = function () {};
            /*jslint emptyblock: false*/

            function init() {
                textSerializer.filter = new odf.OdfNodeFilter();

                linkEditorContent = new EditHyperlinkPane();
                dialog = new TooltipDialog({
                    title: runtime.tr("Edit link"),
                    content: linkEditorContent.widget(),
                    onShow: updateLinkEditorContent
                });

                editHyperlinkButton = new DropDownButton({
                    label: runtime.tr('Edit link'),
                    showLabel: false,
                    disabled: true,
                    iconClass: 'dijitEditorIcon dijitEditorIconCreateLink',
                    dropDown: dialog
                });

                removeHyperlinkButton = new Button({
                    label: runtime.tr('Remove link'),
                    showLabel: false,
                    disabled: true,
                    iconClass: 'dijitEditorIcon dijitEditorIconUnlink',
                    onClick: function () {
                        hyperlinkController.removeHyperlinks();
                        self.onToolDone();
                    }
                });

                linkEditorContent.onSave = function () {
                    var hyperlinkData = linkEditorContent.value();
                    editHyperlinkButton.closeDropDown(false);
                    updateSelectedLink(hyperlinkData);
                    self.onToolDone();
                };

                linkEditorContent.onCancel = function () {
                    editHyperlinkButton.closeDropDown(false);
                    self.onToolDone();
                };

                widget.children = [editHyperlinkButton, removeHyperlinkButton];
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
                callback(widget);
            }
            init();
        };

        return EditHyperlinks;
});
