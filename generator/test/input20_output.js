require('module-name');
var i = require('test').default || require('test');
var j = require('./home/index').j;
var k = require('./home/index').k;
var whole = require('./thing').default || require('./thing');
var part = require('./thing').part;
var que = require('kyoo').q;
var allThings = Object.assign({}, require('manyThings').default, require('manyThings'));
var something = require('./something');