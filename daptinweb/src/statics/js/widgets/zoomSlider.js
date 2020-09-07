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

/*global define, require, gui*/

define("webodf/editor/widgets/zoomSlider", [
    "dijit/form/HorizontalSlider"],

    function (HorizontalSlider) {
        "use strict";

        // The slider zooms from -1 to +1, which corresponds
        // to zoom levels of 1/extremeZoomFactor to extremeZoomFactor.
        return function ZoomSlider(callback) {
            var self = this,
                editorSession,
                slider,
                extremeZoomFactor = 4;

            function updateSlider(zoomLevel) {
                slider.set('value', Math.log(zoomLevel) / Math.log(extremeZoomFactor), false);
            }

            this.setEditorSession = function (session) {
                var zoomHelper;
                if (editorSession) {
                    editorSession.getOdfCanvas().getZoomHelper().unsubscribe(gui.ZoomHelper.signalZoomChanged, updateSlider);
                }

                editorSession = session;
                if (editorSession) {
                    zoomHelper = editorSession.getOdfCanvas().getZoomHelper();
                    zoomHelper.subscribe(gui.ZoomHelper.signalZoomChanged, updateSlider);
                    updateSlider(zoomHelper.getZoomLevel());
                }
                slider.setAttribute('disabled', !editorSession);
            };

            /*jslint emptyblock: true*/
            this.onToolDone = function () {};
            /*jslint emptyblock: false*/

            // init
            function init() {
                slider = new HorizontalSlider({
                    name: 'zoomSlider',
                    disabled: true,
                    value: 0,
                    minimum: -1,
                    maximum: 1,
                    discreteValues: 0.01,
                    intermediateChanges: true,
                    style: {
                        width: '150px',
                        height: '25px',
                        float: 'right'
                    }
                });

                slider.onChange = function (value) {
                    if (editorSession) {
                        editorSession.getOdfCanvas().getZoomHelper().setZoomLevel(Math.pow(extremeZoomFactor, value));
                    }
                    self.onToolDone();
                };

                return callback(slider);
            }

            init();
        };
    });
