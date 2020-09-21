<template>
  <q-page-container>
    <div @dblclick="addNew()" id="grid-snap" style="height: 100vh; width: 100vw; overflow: scroll">
      <div @click="itemSelected({'target': {'id' :item.id}})" v-for="item in items" :style="item.style" :id="item.id"
           class="item drag-drop">
        {{ item }}
      </div>
    </div>
    <q-page-sticky position="right" :offset="[0, 0]">
      <q-btn @click="addNew" size="xs" icon="fas fa-arrow-left"/>
    </q-page-sticky>
    <q-page-sticky v-if="selectedItem !== null" position="bottom" :offset="[0, 0]">
      <q-card>
        <div class="row">
          <div class="col-12">{{ selectedItem.script }}</div>
          <div class="col-12">
            <input @keypress.enter="addItemScript(selectedItem, newScriptLine)" v-model="newScriptLine"/>
          </div>
        </div>
      </q-card>
    </q-page-sticky>
    <q-menu
      touch-position
      context-menu @show="itemSelected"
    >

      <q-list dense style="min-width: 100px">
        <q-item clickable v-close-popup>
          <q-item-section @click="deleteItem">Delete</q-item-section>
        </q-item>
        <q-separator/>
        <q-item clickable>
          <q-item-section>Preferences</q-item-section>
          <q-item-section side>
            <q-icon name="keyboard_arrow_right"/>
          </q-item-section>

          <q-menu anchor="top right" self="top left">
            <q-list>
              <q-item
                v-for="n in 3"
                :key="n"
                dense
                clickable
              >
                <q-item-section>Submenu Label</q-item-section>
                <q-item-section side>
                  <q-icon name="keyboard_arrow_right"/>
                </q-item-section>
                <q-menu auto-close anchor="top right" self="top left">
                  <q-list>
                    <q-item
                      v-for="n in 3"
                      :key="n"
                      dense
                      clickable
                    >
                      <q-item-section>3rd level Label</q-item-section>
                    </q-item>
                  </q-list>
                </q-menu>
              </q-item>
            </q-list>
          </q-menu>

        </q-item>
        <q-separator/>
        <q-item clickable v-close-popup>
          <q-item-section>Quit</q-item-section>
        </q-item>
      </q-list>

    </q-menu>
  </q-page-container>
</template>
<style>
.drag-drop {
  touch-action: none;
  user-select: none;
}

.item {
  width: 200px;
  height: 100px;
  position: absolute;
  border: 1px solid cornflowerblue;
  border-radius: 5px;
  padding: 5px;
}

</style>
<script>
import interact from 'interactjs'


var AXIS_RANGE = 12;
var CORNER_RANGE = 14;
var CORNER_EXCLUDE_AXIS = 8;
var AXIS_EXTRA_RANGE = -6;

var myItems = [];
var currentElement = null;
var offX1, offY1, offX2, offY2;

function getPosition(element) {
  return {
    x: parseFloat(element.getAttribute('data-x')) || 0,
    y: parseFloat(element.getAttribute('data-y')) || 0
  };
}

function isBetween(value, min, length) {
  return min - AXIS_EXTRA_RANGE < value && value < (min + length) + AXIS_EXTRA_RANGE;
}

function getDistance(value1, value2) {
  return Math.abs(value1 - value2);
}

function getSnapCoords(element, axis) {
  var result = {
    isOK: false
  };
  if (currentElement && currentElement !== element) {
    var pos = getPosition(element);
    var cur = getPosition(currentElement);
    var distX1a = getDistance(pos.x, cur.x);
    var distX1b = getDistance(pos.x, cur.x + currentElement.offsetWidth);
    var distX2a = getDistance(pos.x + element.offsetWidth, cur.x);
    var distX2b = getDistance(pos.x + element.offsetWidth, cur.x + currentElement.offsetWidth);
    var distY1a = getDistance(pos.y, cur.y);
    var distY1b = getDistance(pos.y, cur.y + currentElement.offsetHeight);
    var distY2a = getDistance(pos.y + element.offsetHeight, cur.y);
    var distY2b = getDistance(pos.y + element.offsetHeight, cur.y + currentElement.offsetHeight);
    var distXa = Math.min(distX1a, distX2a);
    var distXb = Math.min(distX1b, distX2b);
    var distYa = Math.min(distY1a, distY2a);
    var distYb = Math.min(distY1b, distY2b);
    if (distXa < distXb) {
      result.offX = offX1;
    } else {
      result.offX = offX2
    }
    if (distYa < distYb) {
      result.offY = offY1;
    } else {
      result.offY = offY2
    }
    var distX1 = Math.min(distX1a, distX1b);
    var distX2 = Math.min(distX2a, distX2b);
    var distY1 = Math.min(distY1a, distY1b);
    var distY2 = Math.min(distY2a, distY2b);
    var distX = Math.min(distX1, distX2);
    var distY = Math.min(distY1, distY2);
    var dist = Math.max(distX, distY);
    var acceptAxis = dist > CORNER_EXCLUDE_AXIS;

    result.x = distX1 < distX2 ? pos.x : pos.x + element.offsetWidth;
    result.y = distY1 < distY2 ? pos.y : pos.y + element.offsetHeight;

    var inRangeX1 = isBetween(pos.x, cur.x, currentElement.offsetWidth);
    var inRangeX2 = isBetween(cur.x, pos.x, element.offsetWidth);
    var inRangeY1 = isBetween(pos.y, cur.y, currentElement.offsetHeight);
    var inRangeY2 = isBetween(cur.y, pos.y, element.offsetHeight);

    switch (axis) {
      case "x":
        result.isOK = acceptAxis && (inRangeY1 || inRangeY2);
        break;
      case "y":
        result.isOK = acceptAxis && (inRangeX1 || inRangeX2);
        break;
      default:
        result.isOK = true;
        break;
    }
  }
  return result;
}

const gridWidth = document.body.clientWidth / 12;


export default {
  name: "DragEditor",
  data() {
    return {
      items: [],
      n: 0,
      newScriptLine: "",
      selectedItem: null,
    }
  },
  methods: {
    addItemScript(item, line) {
      console.log("add item script")
      var indx = line.indexOf(":")
      if (indx === -1) {
        item.tag = line.trim()
      } else {
        var lineParts = line.split(":")
        var key = lineParts[0].trim()
        var keyParts = key.split(".")
        var value = lineParts[1].trim()
        let props = item.props;
        for (let i = 0; i < keyParts.length; i++) {
          if (i + 1 === keyParts.length) {
            props[keyParts[i]] = value;
            break
          }
          if (!props[keyParts[i]]) {
            props[keyParts[i]] = {}
          }
          props = props[keyParts[i]]
        }
      }
      // item.script = item.script + line + ";\n";
      this.newScriptLine = "";
    },
    deleteItem(item) {
      console.log("Delete item ", this.selectedItem);
      this.selectedItem = null;
      let i = -1;
      for (var j = 0; j < this.items.length; j++) {
        if (this.items[j].id === this.selectedItem.id) {
          i = j;
          break;
        }
      }
      if (i !== -1) {
        this.items.splice(i, 1)
        this.selectedItem = null;
      }
    },
    getItemById(id) {
      return this.items.filter(function (r) {
        return r.id === id;
      })[0];
    },
    getItemContainerTargetBtId(id) {
      return document.getElementById(id);
    },
    itemSelected(event) {
      console.log("Item selected", event)
      this.selectedItem = this.getItemById(event.target.id);
    },
    addNew() {
      console.log("add new", arguments);

      this.n += 1;
      this.items.push({
        name: "item " + this.n,
        id: "item-" + this.n,
        style: {
          top: "0",
          left: "0",
          width: (2 * gridWidth) + "px",
          height: gridWidth + "px",
        }
      });
    }
  },
  mounted() {
    console.log("Mounted drag editor");
    const position = {x: document.body.clientWidth / 2, y: document.body.clientHeight / 2}
    const that = this;

    // var element = document.getElementById('grid-snap')

    var gridWidth = document.body.clientWidth / 12;

    interact('.drag-drop')
      .draggable({
        inertia: true,
        modifiers: [
          interact.modifiers.snap({
            targets: [
              interact.createSnapGrid({x: gridWidth, y: gridWidth})
            ],
            range: Infinity,
            relativePoints: [{x: 0, y: 0}]
          }),
          interact.modifiers.restrict({
            restriction: "#grid-snap",
            elementRect: {top: 0, left: 0, bottom: 1, right: 1},
            endOnly: false
          })
        ],
        listeners: {
          move(event) {
            var rectangle = that.items.filter(function (e) {
              return e.id === event.target.id
            })[0];

            var left = rectangle.style.left;
            var top = rectangle.style.top;
            if (left.endsWith && left.endsWith("px")) {
              left = left.substring(0, left.length - 2)
            }
            if (top.endsWith && top.endsWith("px")) {
              top = top.substring(0, top.length - 2)
            }
            left = parseFloat(left) || 0;
            top = parseFloat(top) || 0;
            left += event.dx;
            top += event.dy;
            left = parseInt(left)
            top = parseInt(top)
            // console.log("on move", event, rectangle)
            rectangle.style.left = left + "px";
            rectangle.style.top = top + "px";
          }
        },
      })
      .resizable({
        edges: {left: true, right: true, top: true, bottom: true},
        modifiers: [
          interact.modifiers.snapSize({
            targets: [
              {width: gridWidth},
              interact.createSnapGrid({width: gridWidth, height: gridWidth})
            ]
          })
        ],
        invert: 'reposition',
        onmove: function (event) {
          var rectangle = that.items.filter(function (e) {
            return e.id === event.target.id
          })[0];
          rectangle.style.left = event.rect.left + "px";
          rectangle.style.top = event.rect.top + "px";
          rectangle.style.width = event.rect.width + "px";
          rectangle.style.height = event.rect.height + "px";
        }
      });

    this.addNew();


  }
}
</script>
