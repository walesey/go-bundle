module.exports = k;
module.exports.default = i;
module.exports.j = 'test';
module.exports.fn = (function () {
  return console.log('arrow fn');
});
module.exports.fn2 = (function (a, b) {
  return (a + b);
});