// scripts/notes-obsidian-markdown.test.mjs
import test from 'node:test';
import assert from 'node:assert/strict';
import {fromMarkdown} from 'mdast-util-from-markdown';
import {toMarkdown} from 'mdast-util-to-markdown';
import {gfm} from 'micromark-extension-gfm';
import {gfmFromMarkdown, gfmToMarkdown} from 'mdast-util-gfm';
import {obsidianSyntax, obsidianFromMarkdown, obsidianToMarkdown, parseWikiValue} from './notes-obsidian-markdown.js';

const parse = text => fromMarkdown(text, {extensions: [gfm(), obsidianSyntax()], mdastExtensions: [gfmFromMarkdown(), obsidianFromMarkdown()]});
const stringify = tree => toMarkdown(tree, {bullet: '-', extensions: [gfmToMarkdown(), obsidianToMarkdown()]});
const all = (node, type, out = []) => { if (node.type === type) out.push(node); for (const child of node.children || []) all(child, type, out); return out; };
const same = text => assert.equal(stringify(parse(text)), text);

test('wikilinks parse with raw value and round-trip verbatim', () => {
    const tree = parse('See [[Folder/Note#Heading|Alias]] and [[Plain]].\n');
    assert.deepEqual(all(tree, 'wikiLink').map(n => [n.value, n.embed]), [['Folder/Note#Heading|Alias', false], ['Plain', false]]);
    same('See [[Folder/Note#Heading|Alias]] and [[Plain]].\n');
    same('[[Note]] starts the line\n');
    same('Embed ![[image.png]] inline\n');
    same('![[Other note#Part]]\n');
    assert.deepEqual(all(parse('![[image.png]]\n'), 'wikiLink').map(n => [n.value, n.embed]), [['image.png', true]]);
});

test('wikilinks inside table cells keep the escaped pipe', () => {
    const tree = parse('| Link |\n| - |\n| [[Note\\|Alias]] |\n');
    assert.deepEqual(all(tree, 'wikiLink').map(n => n.value), ['Note\\|Alias']);
    const out = stringify(tree);
    assert.ok(out.includes('[[Note\\|Alias]]'), out);
    assert.deepEqual(all(parse(out), 'wikiLink').map(n => n.value), ['Note\\|Alias']);
    assert.deepEqual(parseWikiValue('Note\\|Alias'), {target: 'Note', subpath: '', alias: 'Alias'});
});

test('parseWikiValue splits target, subpath and alias', () => {
    assert.deepEqual(parseWikiValue('Folder/Note#Heading|Alias'), {target: 'Folder/Note', subpath: 'Heading', alias: 'Alias'});
    assert.deepEqual(parseWikiValue('Note#^block-1'), {target: 'Note', subpath: '^block-1', alias: ''});
    assert.deepEqual(parseWikiValue('#Local heading'), {target: '', subpath: 'Local heading', alias: ''});
    assert.deepEqual(parseWikiValue(' Spaced | Shown '), {target: 'Spaced', subpath: '', alias: 'Shown'});
});

test('non-wikilinks stay literal', () => {
    for (const text of ['\\[[x]]\n', 'a [[b\nc]] d\n', '`[[code]]`\n', '```\n[[fenced]]\n```\n', '[[]]\n']) {
        const tree = parse(text);
        assert.equal(all(tree, 'wikiLink').length, 0, text);
        assert.equal(all(parse(stringify(tree)), 'wikiLink').length, 0, 'reparse ' + text);
    }
    same('```\n[[fenced]]\n```\n');
    // Like Obsidian: an unclosed outer [[ stays literal, the inner link still works.
    assert.deepEqual(all(parse('[[a [[b]]\n'), 'wikiLink').map(n => n.value), ['b']);
});

test('hashtags parse at word starts and never get escaped', () => {
    same('#tag at the start\n');
    const tree = parse('Text #projekt/sub-a_b and #über (#paren)\n');
    assert.deepEqual(all(tree, 'hashtag').map(n => n.value), ['projekt/sub-a_b', 'über', 'paren']);
    same('Text #projekt/sub-a_b and #über (#paren)\n');
    assert.deepEqual(all(parse('#12a ok\n'), 'hashtag').map(n => n.value), ['12a']);
    for (const text of ['Issue #123\n', 'a#b\n', '# Heading\n', 'http://example.com/#frag\n', '`#code`\n']) {
        assert.equal(all(parse(text), 'hashtag').length, 0, text);
    }
});

test('callout markers survive in blockquotes', () => {
    const tree = parse('> [!warning]- Careful\n> Body text\n');
    assert.deepEqual(all(tree, 'calloutMarker').map(n => [n.value, n.fold]), [['warning', '-']]);
    same('> [!warning]- Careful\n> Body text\n');
    same('> [!note]\n> Body\n');
    same('> [!tip]+ Open by default\n');
    assert.equal(all(parse('[!x](https://example.com)\n'), 'calloutMarker').length, 0);
    assert.equal(all(parse('text [!x] mid\n'), 'calloutMarker').length, 0);
});

test('highlights allow nested phrasing and line starts', () => {
    const tree = parse('==marked== and ==**bold** inside==\n');
    const marks = all(tree, 'highlight');
    assert.equal(marks.length, 2);
    assert.equal(all(marks[1], 'strong').length, 1);
    same('==marked== and ==**bold** inside==\n');
    same('==start== of a line\n');
    for (const text of ['a == b\n', '===\n', 'x ==not closed\n']) assert.equal(all(parse(text), 'highlight').length, 0, text);
});

test('a full Obsidian note round-trips unchanged', () => {
    same('# Projekt\n\nVerweis auf [[Ideen|meine Ideen]] und #projekt/alpha.\n\n> [!info] Hinweis\n> Siehe ==wichtig== und ![[skizze.png]]\n\n- [ ] Aufgabe mit [[Aufgaben#Heute]]\n');
});
