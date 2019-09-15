"use strict";
/**
 * @license
 * Copyright Google Inc. All Rights Reserved.
 *
 * Use of this source code is governed by an MIT-style license that can be
 * found in the LICENSE file at https://angular.io/license
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.ERROR_TYPE = 'ngType';
exports.ERROR_DEBUG_CONTEXT = 'ngDebugContext';
exports.ERROR_ORIGINAL_ERROR = 'ngOriginalError';
exports.ERROR_LOGGER = 'ngErrorLogger';
function getType(error) {
    return error[exports.ERROR_TYPE];
}
exports.getType = getType;
function getDebugContext(error) {
    return error[exports.ERROR_DEBUG_CONTEXT];
}
exports.getDebugContext = getDebugContext;
function getOriginalError(error) {
    return error[exports.ERROR_ORIGINAL_ERROR];
}
exports.getOriginalError = getOriginalError;
function getErrorLogger(error) {
    return error[exports.ERROR_LOGGER] || defaultErrorLogger;
}
exports.getErrorLogger = getErrorLogger;
function defaultErrorLogger(console) {
    var values = [];
    for (var _i = 1; _i < arguments.length; _i++) {
        values[_i - 1] = arguments[_i];
    }
    console.error.apply(console, values);
}
//# sourceMappingURL=errors.js.map