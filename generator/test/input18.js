const arrowFn = () => {
  var i = 'arrow';
  console.log(i);
};

[1, 2].reduce((i, acc) => {
  if (i > 1) {
    return acc.push(i);
  } 
  return acc
}, []);

[1,2,3,4].map(i => i+3);