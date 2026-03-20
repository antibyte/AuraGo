const gulp = require('gulp');
const svgmin = require('gulp-svgmin');

function minifyIcons() {
	return gulp
		.src('nodes/icons/*.svg')
		.pipe(svgmin())
		.pipe(gulp.dest('dist/nodes/icons'));
}

function copyIcons() {
	return gulp
		.src('nodes/icons/*.svg')
		.pipe(gulp.dest('dist/nodes/icons'));
}

exports['build:icons'] = gulp.series(minifyIcons, copyIcons);
