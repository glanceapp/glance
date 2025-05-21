export function elem(tag = "div") {
    return document.createElement(tag);
}

export function fragment(...children) {
    const f = document.createDocumentFragment();
    if (children) f.append(...children);
    return f;
}

export function text(str = "") {
    return document.createTextNode(str);
}

export function repeat(n, fn) {
    const elems = Array(n);

    for (let i = 0; i < n; i++)
        elems[i] = fn(i);

    return elems;
}

export function find(selector) {
    return document.querySelector(selector);
}

export function findAll(selector) {
    return document.querySelectorAll(selector);
}


HTMLCollection.prototype.map = function(fn) {
    return Array.from(this).map(fn);
}

HTMLCollection.prototype.indexOf = function(element) {
    return Array.prototype.indexOf.call(this, element);
}

const ep = HTMLElement.prototype;
const fp = DocumentFragment.prototype;
const tp = Text.prototype;

ep.classes = function(...classes) {
    this.classList.add(...classes);
    return this;
}

ep.find = function(selector) {
    return this.querySelector(selector);
}

ep.findAll = function(selector) {
    return this.querySelectorAll(selector);
}

ep.classesIf = function(cond, ...classes) {
    cond ? this.classList.add(...classes) : this.classList.remove(...classes);
    return this;
}

ep.hide = function() {
    this.style.display = "none";
    return this;
}

ep.show = function() {
    this.style.removeProperty("display");
    return this;
}

ep.showIf = function(cond) {
    cond ? this.show() : this.hide();
    return this;
}

ep.isHidden = function() {
    return this.style.display === "none";
}

ep.clearClasses = function(...classes) {
    classes.length ? this.classList.remove(...classes) : this.className = "";
    return this;
}

ep.hasClass = function(className) {
    return this.classList.contains(className);
}

ep.attr = function(name, value) {
    this.setAttribute(name, value);
    return this;
}

ep.attrs = function(attrs) {
    for (const [name, value] of Object.entries(attrs))
        this.setAttribute(name, value);
    return this;
}

ep.tap = function(fn) {
    fn(this);
    return this;
}

ep.text = function(text) {
    this.innerText = text;
    return this;
}

ep.html = function(html) {
    this.innerHTML = html;
    return this;
}

ep.appendTo = function(parent) {
    parent.appendChild(this);
    return this;
}

ep.swapWith = function(element) {
    this.replaceWith(element);
    return element;
}

ep.on = function(event, callback, options) {
    if (typeof event === "string") {
        this.addEventListener(event, callback, options);
        return this;
    }

    for (let i = 0; i < event.length; i++)
        this.addEventListener(event[i], callback, options);

    return this;
}

const epAppend = ep.append;
ep.append = function(...children) {
    epAppend.apply(this, children);
    return this;
}

ep.duplicate = function(n) {
    const elems = Array(n);

    for (let i = 0; i < n; i++)
        elems[i] = this.cloneNode(true);

    return elems;
}

ep.styles = function(s) {
    Object.assign(this.style, s);
    return this;
}

ep.clearStyles = function(...props) {
    for (let i = 0; i < props.length; i++)
        this.style.removeProperty(props[i]);
    return this;
}

ep.disable = function() {
    this.disabled = true;
    return this;
}

ep.enable = function() {
    this.disabled = false;
    return this;
}

const epAnimate = ep.animate;
ep.animate = function(anim, callback) {
    const a = epAnimate.call(this, anim.keyframes, anim.options);
    if (callback) a.onfinish = () => callback(this, a);
    return this;
}

ep.animateUpdate = function(update, exit, entrance) {
    this.animate(exit, () => {
        update(this);
        this.animate(entrance);
    });

    return this;
}

ep.styleVar = function(name, value) {
    this.style.setProperty(`--${name}`, value);
    return this;
}

ep.component = function (methods) {
    this.component = methods;
    return this;
}

const fpAppend = fp.append;
fp.append = function(...children) {
    fpAppend.apply(this, children);
    return this;
}

fp.appendTo = function(parent) {
    parent.appendChild(this);
    return this;
}

tp.text = function(text) {
    this.nodeValue = text;
    return this;
}
