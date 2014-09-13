module.exports = function(grunt) {
	grunt.initConfig({
		pkg: grunt.file.readJSON('package.json'),
		exec: {
			push: 'subliminl push <%= grunt.file.readJSON("package.json").name %>:<%= grunt.file.readJSON("package.json").version %>'
		}
	});
	grunt.loadNpmTasks('grunt-dib');
	grunt.loadNpmTasks('grunt-exec');
	grunt.loadNpmTasks('grunt-release');

	// Build task
	grunt.registerTask('build', ['dib']);

	// Build a release
	grunt.registerTask('release', ['git-release', 'dib'])

	// Push image to registry
	grunt.registerTask('push', ['check-deploy', 'exec:push'])

	// Default task
	grunt.registerTask('default', ['build']);
};