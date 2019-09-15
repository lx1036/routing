"use strict";
/**
 * @license
 * Copyright Google Inc. All Rights Reserved.
 *
 * Use of this source code is governed by an MIT-style license that can be
 * found in the LICENSE file at https://angular.io/license
 */
exports.__esModule = true;
/**
 * Provides access to reflection data about symbols. Used internally by Angular
 * to power dependency injection and compilation.
 */
var Reflector = /** @class */ (function () {
    function Reflector(reflectionCapabilities) {
        this.reflectionCapabilities = reflectionCapabilities;
    }
    Reflector.prototype.updateCapabilities = function (caps) { this.reflectionCapabilities = caps; };
    Reflector.prototype.factory = function (type) { return this.reflectionCapabilities.factory(type); };
    Reflector.prototype.parameters = function (typeOrFunc) {
        return this.reflectionCapabilities.parameters(typeOrFunc);
    };
    Reflector.prototype.annotations = function (typeOrFunc) {
        return this.reflectionCapabilities.annotations(typeOrFunc);
    };
    Reflector.prototype.propMetadata = function (typeOrFunc) {
        return this.reflectionCapabilities.propMetadata(typeOrFunc);
    };
    Reflector.prototype.hasLifecycleHook = function (type, lcProperty) {
        return this.reflectionCapabilities.hasLifecycleHook(type, lcProperty);
    };
    Reflector.prototype.getter = function (name) { return this.reflectionCapabilities.getter(name); };
    Reflector.prototype.setter = function (name) { return this.reflectionCapabilities.setter(name); };
    Reflector.prototype.method = function (name) { return this.reflectionCapabilities.method(name); };
    Reflector.prototype.importUri = function (type) { return this.reflectionCapabilities.importUri(type); };
    Reflector.prototype.resourceUri = function (type) { return this.reflectionCapabilities.resourceUri(type); };
    Reflector.prototype.resolveIdentifier = function (name, moduleUrl, members, runtime) {
        return this.reflectionCapabilities.resolveIdentifier(name, moduleUrl, members, runtime);
    };
    Reflector.prototype.resolveEnum = function (identifier, name) {
        return this.reflectionCapabilities.resolveEnum(identifier, name);
    };
    return Reflector;
}());
exports.Reflector = Reflector;
