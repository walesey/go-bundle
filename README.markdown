# Risotto
Package risotto is a JavaScript to JavaScript compiler. Risotto's parser and AST is forked from [otto](https://github.com/robertkrimen/otto).
The main motivation behind Risotto is to be used by [Gonads](https://github.com/mamaar/gonads), a frontend toolkit that currently compiles JSX and SASS.

# Example
### Input JavaScript
```
(function() {
    var i = <div />;
    console.log("Hello, world!")
})
```


### Output JavaScript
```
(function () {
    var i = React.createElement("div", null);
    console.log("Hello, world!");
});
```
