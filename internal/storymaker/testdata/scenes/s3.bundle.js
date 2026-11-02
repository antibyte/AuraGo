var __smSceneModule = (() => {
  var __defProp = Object.defineProperty;
  var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
  var __getOwnPropNames = Object.getOwnPropertyNames;
  var __hasOwnProp = Object.prototype.hasOwnProperty;
  var __export = (target, all) => {
    for (var name in all)
      __defProp(target, name, { get: all[name], enumerable: true });
  };
  var __copyProps = (to, from, except, desc) => {
    if (from && typeof from === "object" || typeof from === "function") {
      for (let key of __getOwnPropNames(from))
        if (!__hasOwnProp.call(to, key) && key !== except)
          __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
    }
    return to;
  };
  var __toCommonJS = (mod) => __copyProps(__defProp({}, "__esModule", { value: true }), mod);
  var s3_exports = {};
  __export(s3_exports, {
    default: () => scene
  });
  function scene(api) {
    api.duration(5);
    const anna = api.add("char.office-woman-1", { at: "left", pose: "stand" });
    const label = api.text("3 von 4 Paketen geblockt", "title");
    anna.gesture("point", { start: 1 });
    api.tween(label, { opacity: 1 }, { start: 0.5, dur: 1, ease: "ease-out" });
    anna.emote("happy", { start: 3 });
  }
  return __toCommonJS(s3_exports);
})();

self.__smScene=__smSceneModule.default;
