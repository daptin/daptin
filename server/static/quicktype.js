#!/usr/bin/env node
"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
var __generator = (this && this.__generator) || function (thisArg, body) {
    var _ = { label: 0, sent: function() { if (t[0] & 1) throw t[1]; return t[1]; }, trys: [], ops: [] }, f, y, t, g;
    return g = { next: verb(0), "throw": verb(1), "return": verb(2) }, typeof Symbol === "function" && (g[Symbol.iterator] = function() { return this; }), g;
    function verb(n) { return function (v) { return step([n, v]); }; }
    function step(op) {
        if (f) throw new TypeError("Generator is already executing.");
        while (_) try {
            if (f = 1, y && (t = y[op[0] & 2 ? "return" : op[0] ? "throw" : "next"]) && !(t = t.call(y, op[1])).done) return t;
            if (y = 0, t) op = [0, t.value];
            switch (op[0]) {
                case 0: case 1: t = op; break;
                case 4: _.label++; return { value: op[1], done: false };
                case 5: _.label++; y = op[1]; op = [0]; continue;
                case 7: op = _.ops.pop(); _.trys.pop(); continue;
                default:
                    if (!(t = _.trys, t = t.length > 0 && t[t.length - 1]) && (op[0] === 6 || op[0] === 2)) { _ = 0; continue; }
                    if (op[0] === 3 && (!t || (op[1] > t[0] && op[1] < t[3]))) { _.label = op[1]; break; }
                    if (op[0] === 6 && _.label < t[1]) { _.label = t[1]; t = op; break; }
                    if (t && _.label < t[2]) { _.label = t[2]; _.ops.push(op); break; }
                    if (t[2]) _.ops.pop();
                    _.trys.pop(); continue;
            }
            op = body.call(thisArg, _);
        } catch (e) { op = [6, e]; y = 0; } finally { f = t = 0; }
        if (op[0] & 5) throw op[1]; return { value: op[0] ? op[1] : void 0, done: true };
    }
};
Object.defineProperty(exports, "__esModule", { value: true });
var fs = require("fs");
var path = require("path");
var process = require("process");
var Either = require("./either");
var try_require_1 = require("./try-require");
var _ = require("lodash");
var Main = try_require_1.default("../output/Main", "./Main");
var makeSource = require("stream-json");
var Assembler = require("stream-json/utils/Assembler");
var commandLineArgs = require('command-line-args');
var getUsage = require('command-line-usage');
var fetch = require("node-fetch");
var chalk = require("chalk");
var langs = Main.renderers.map(function (r) { return r.extension; }).join("|");
var langNames = Main.renderers.map(function (r) { return r.name; }).join(", ");
var optionDefinitions = [
    {
        name: 'out',
        alias: 'o',
        type: String,
        typeLabel: "FILE",
        description: 'The output file. Determines --lang and --top-level.'
    },
    {
        name: 'top-level',
        alias: 't',
        type: String,
        typeLabel: 'NAME',
        description: 'The name for the top level type.'
    },
    {
        name: 'lang',
        alias: 'l',
        type: String,
        typeLabel: langs,
        description: 'The target language.'
    },
    {
        name: 'src-lang',
        alias: 's',
        type: String,
        defaultValue: 'json',
        typeLabel: 'json|schema',
        description: 'The source language (default is json).'
    },
    {
        name: 'src',
        type: String,
        multiple: true,
        defaultOption: true,
        typeLabel: 'FILE|URL',
        description: 'The file or url to type.'
    },
    {
        name: 'src-urls',
        type: String,
        typeLabel: 'FILE',
        description: 'Tracery grammar describing URLs to crawl.'
    },
    {
        name: 'help',
        alias: 'h',
        type: Boolean,
        description: 'Get some help.'
    }
];
var sectionsBeforeRenderers = [
    {
        header: 'Synopsis',
        content: "$ quicktype [[bold]{--lang} " + langs + "] FILE|URL ..."
    },
    {
        header: 'Description',
        content: "Given JSON sample data, quicktype outputs code for working with that data in " + langNames + "."
    },
    {
        header: 'Options',
        optionList: optionDefinitions
    }
];
var sectionsAfterRenderers = [
    {
        header: 'Examples',
        content: [
            chalk.dim('Generate C# to parse a Bitcoin API'),
            '$ quicktype -o LatestBlock.cs https://blockchain.info/latestblock',
            '',
            chalk.dim('Generate Go code from a JSON file'),
            '$ quicktype -l go user.json',
            '',
            chalk.dim('Generate JSON Schema, then TypeScript'),
            '$ quicktype -o schema.json https://blockchain.info/latestblock',
            '$ quicktype -o bitcoin.ts --src-lang schema schema.json'
        ]
    },
    {
        content: 'Learn more at [bold]{quicktype.io}'
    }
];
function optionDefinitionsForRenderer(renderer) {
    return _.map(renderer.options, function (o) {
        return {
            name: o.name,
            description: o.description,
            typeLabel: o.typeLabel,
            renderer: true,
            type: String
        };
    });
}
function usage() {
    var rendererSections = [];
    _.forEach(Main.renderers, function (renderer) {
        if (renderer.options.length == 0)
            return;
        rendererSections.push({
            header: "Options for " + renderer.name,
            optionList: optionDefinitionsForRenderer(renderer)
        });
    });
    var sections = _.concat(sectionsBeforeRenderers, rendererSections, sectionsAfterRenderers);
    console.log(getUsage(sections));
}
var Run = /** @class */ (function () {
    function Run(argv) {
        var _this = this;
        this.getRenderer = function (lang) {
            var renderer = Main.renderers.find(function (r) { return _.includes(r, lang); });
            if (!renderer) {
                console.error("'" + _this.options.lang + "' is not yet supported as an output language.");
                process.exit(1);
            }
            return renderer;
        };
        this.renderSamplesOrSchemas = function (samplesOrSchemas) {
            var areSchemas = _this.options.srcLang === "schema";
            var config = {
                language: _this.getRenderer(_this.options.lang).extension,
                topLevels: Object.getOwnPropertyNames(samplesOrSchemas).map(function (name) {
                    if (areSchemas) {
                        // Only one schema per top-level is used right now
                        return { name: name, schema: samplesOrSchemas[name][0] };
                    }
                    else {
                        return { name: name, samples: samplesOrSchemas[name] };
                    }
                }),
                rendererOptions: _this.rendererOptions
            };
            return Either.fromRight(Main.main(config));
        };
        this.splitAndWriteJava = function (dir, str) {
            var lines = str.split("\n");
            var filename = null;
            var currentFileContents = "";
            var writeFile = function () {
                if (filename != null) {
                    fs.writeFileSync(path.join(dir, filename), currentFileContents);
                }
                filename = null;
                currentFileContents = "";
            };
            var i = 0;
            while (i < lines.length) {
                var line = lines[i];
                i += 1;
                var results = line.match("^// (.+\\.java)$");
                if (results == null) {
                    currentFileContents += line + "\n";
                }
                else {
                    writeFile();
                    filename = results[1];
                    while (lines[i] == "")
                        i++;
                }
            }
            writeFile();
        };
        this.renderAndOutput = function (samplesOrSchemas) {
            var output = _this.renderSamplesOrSchemas(samplesOrSchemas);
            if (_this.options.out) {
                if (_this.options.lang == "java") {
                    _this.splitAndWriteJava(path.dirname(_this.options.out), output);
                }
                else {
                    fs.writeFileSync(_this.options.out, output);
                }
            }
            else {
                process.stdout.write(output);
            }
        };
        this.workFromJsonArray = function (jsonArray) {
            var map = {};
            map[_this.options.topLevel] = jsonArray;
            _this.renderAndOutput(map);
        };
        this.parseJsonFromStream = function (stream) {
            return new Promise(function (resolve) {
                var source = makeSource();
                var assembler = new Assembler();
                var assemble = function (chunk) { return assembler[chunk.name] && assembler[chunk.name](chunk.value); };
                var isInt = function (intString) { return /^\d+$/.test(intString); };
                var intSentinelChunks = function (intString) { return [
                    { name: 'startObject' },
                    { name: 'startKey' },
                    { name: 'stringChunk', value: Main.intSentinel },
                    { name: 'endKey' },
                    { name: 'keyValue', value: Main.intSentinel },
                    { name: 'startNumber' },
                    { name: 'numberChunk', value: intString },
                    { name: 'endNumber' },
                    { name: 'numberValue', value: intString },
                    { name: 'endObject' }
                ]; };
                var queue = [];
                source.output.on("data", function (chunk) {
                    switch (chunk.name) {
                        case "startNumber":
                        case "numberChunk":
                        case "endNumber":
                            // We queue number chunks until we decide if they are int
                            queue.push(chunk);
                            break;
                        case "numberValue":
                            queue.push(chunk);
                            if (isInt(chunk.value)) {
                                intSentinelChunks(chunk.value).forEach(assemble);
                            }
                            else {
                                queue.forEach(assemble);
                            }
                            queue = [];
                            break;
                        default:
                            assemble(chunk);
                    }
                });
                source.output.on("end", function () { return resolve(assembler.current); });
                stream.setEncoding('utf8');
                stream.pipe(source.input);
                stream.resume();
            });
        };
        this.mapValues = function (obj, f) { return __awaiter(_this, void 0, void 0, function () {
            var result, _i, _a, key, _b, _c;
            return __generator(this, function (_d) {
                switch (_d.label) {
                    case 0:
                        result = {};
                        _i = 0, _a = Object.keys(obj);
                        _d.label = 1;
                    case 1:
                        if (!(_i < _a.length)) return [3 /*break*/, 4];
                        key = _a[_i];
                        _b = result;
                        _c = key;
                        return [4 /*yield*/, f(obj[key])];
                    case 2:
                        _b[_c] = _d.sent();
                        _d.label = 3;
                    case 3:
                        _i++;
                        return [3 /*break*/, 1];
                    case 4: return [2 /*return*/, result];
                }
            });
        }); };
        this.parseFileOrUrl = function (fileOrUrl) { return __awaiter(_this, void 0, void 0, function () {
            var res;
            return __generator(this, function (_a) {
                switch (_a.label) {
                    case 0:
                        if (!fs.existsSync(fileOrUrl)) return [3 /*break*/, 1];
                        return [2 /*return*/, this.parseJsonFromStream(fs.createReadStream(fileOrUrl))];
                    case 1: return [4 /*yield*/, fetch(fileOrUrl)];
                    case 2:
                        res = _a.sent();
                        return [2 /*return*/, this.parseJsonFromStream(res.body)];
                }
            });
        }); };
        this.parseFileOrUrlArray = function (filesOrUrls) {
            return Promise.all(filesOrUrls.map(_this.parseFileOrUrl));
        };
        this.main = function () { return __awaiter(_this, void 0, void 0, function () {
            var json, jsonMap, _a, json, jsons;
            return __generator(this, function (_b) {
                switch (_b.label) {
                    case 0:
                        if (!this.options.help) return [3 /*break*/, 1];
                        usage();
                        return [3 /*break*/, 8];
                    case 1:
                        if (!this.options.srcUrls) return [3 /*break*/, 3];
                        json = JSON.parse(fs.readFileSync(this.options.srcUrls, "utf8"));
                        jsonMap = Either.fromRight(Main.urlsFromJsonGrammar(json));
                        _a = this.renderAndOutput;
                        return [4 /*yield*/, this.mapValues(jsonMap, this.parseFileOrUrlArray)];
                    case 2:
                        _a.apply(this, [_b.sent()]);
                        return [3 /*break*/, 8];
                    case 3:
                        if (!(this.options.src.length == 0)) return [3 /*break*/, 5];
                        return [4 /*yield*/, this.parseJsonFromStream(process.stdin)];
                    case 4:
                        json = _b.sent();
                        this.workFromJsonArray([json]);
                        return [3 /*break*/, 8];
                    case 5:
                        if (!(this.options.src.length == 1)) return [3 /*break*/, 7];
                        return [4 /*yield*/, this.parseFileOrUrlArray(this.options.src)];
                    case 6:
                        jsons = _b.sent();
                        this.workFromJsonArray(jsons);
                        return [3 /*break*/, 8];
                    case 7:
                        usage();
                        process.exit(1);
                        _b.label = 8;
                    case 8: return [2 /*return*/];
                }
            });
        }); };
        // Parse the options in argv and split them into global options and renderer options,
        // according to each option definition's `renderer` field.  If `partial` is false this
        // will throw if it encounters an unknown option.
        this.parseOptions = function (optionDefinitions, argv, partial) {
            var opts = commandLineArgs(optionDefinitions, { argv: argv, partial: partial });
            var options = {};
            var renderer = {};
            optionDefinitions.forEach(function (o) {
                if (!(o.name in opts))
                    return;
                var v = opts[o.name];
                if (o.renderer)
                    renderer[o.name] = v;
                else {
                    var k = _.lowerFirst(o.name.split('-').map(_.upperFirst).join(''));
                    options[k] = v;
                }
            });
            return { options: _this.inferOptions(options), renderer: renderer };
        };
        this.inferOptions = function (opts) {
            opts.src = opts.src || [];
            opts.srcLang = opts.srcLang || "json";
            opts.lang = opts.lang || _this.inferLang(opts);
            opts.topLevel = opts.topLevel || _this.inferTopLevel(opts);
            return opts;
        };
        this.inferLang = function (options) {
            // Output file extension determines the language if language is undefined
            if (options.out) {
                var extension = path.extname(options.out);
                if (extension == "") {
                    console.error("Please specify a language (--lang) or an output file extension.");
                    process.exit(1);
                }
                return extension.substr(1);
            }
            return "go";
        };
        this.inferTopLevel = function (options) {
            // Output file name determines the top-level if undefined
            if (options.out) {
                var extension = path.extname(options.out);
                var without = path.basename(options.out).replace(extension, "");
                return without;
            }
            // Source determines the top-level if undefined
            if (options.src.length == 1) {
                var src = options.src[0];
                var extension = path.extname(src);
                var without = path.basename(src).replace(extension, "");
                return without;
            }
            return "TopLevel";
        };
        if (_.isArray(argv)) {
            // We can only fully parse the options once we know which renderer is selected,
            // because there are renderer-specific options.  But we only know which renderer
            // is selected after we've parsed the options.  Hence, we parse the options
            // twice.  This is the first parse to get the renderer:
            var incompleteOptions = this.parseOptions(optionDefinitions, argv, true).options;
            var renderer = this.getRenderer(incompleteOptions.lang);
            // Use the global options as well as the renderer options from now on:
            var rendererOptionDefinitions = optionDefinitionsForRenderer(renderer);
            var allOptionDefinitions = _.concat(optionDefinitions, rendererOptionDefinitions);
            try {
                // This is the parse that counts:
                var _a = this.parseOptions(allOptionDefinitions, argv, false), options = _a.options, rendererOptions = _a.renderer;
                this.options = options;
                this.rendererOptions = rendererOptions;
            }
            catch (error) {
                if (error.name === 'UNKNOWN_OPTION') {
                    console.error("Error: Unknown option");
                    usage();
                    process.exit(1);
                }
                throw error;
            }
        }
        else {
            this.options = this.inferOptions(argv);
            this.rendererOptions = {};
        }
    }
    return Run;
}());
function main(args) {
    return __awaiter(this, void 0, void 0, function () {
        var run;
        return __generator(this, function (_a) {
            switch (_a.label) {
                case 0:
                    if (!(_.isArray(args) && args.length == 0)) return [3 /*break*/, 1];
                    usage();
                    return [3 /*break*/, 3];
                case 1:
                    run = new Run(args);
                    return [4 /*yield*/, run.main()];
                case 2:
                    _a.sent();
                    _a.label = 3;
                case 3: return [2 /*return*/];
            }
        });
    });
}
exports.main = main;
if (require.main === module) {
    main(process.argv.slice(2)).catch(function (reason) {
        console.error(reason);
        process.exit(1);
    });
}
