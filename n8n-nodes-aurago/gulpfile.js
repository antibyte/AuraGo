const gulp = require('gulp');
const svgmin = require('gulp-svgmin');

function minifyIcons() {
	return gulp
		.src('nodes/**/*.svg')
		.pipe(svgmin())
		.pipe(gulp.dest('dist/nodes'));
}

function copyIcons() {
	return gulp
		.src('nodes/**/*.svg')
		.pipe(gulp.dest('dist/nodes'));
}

exports['build:icons'] = gulp.series(minifyIcons, copyIcons);
