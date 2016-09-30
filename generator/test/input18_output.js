var arrowFn = (function () {
  var i = 'arrow';
  console.log(i);
});
[1, 2].reduce((function (i, acc) {
  if ((i > 1)) {
    return acc.push(i);
  }
  return acc;
}), []);
[1, 2, 3, 4].map((function (z) {
  return (z + 3);
}));