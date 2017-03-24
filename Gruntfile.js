module.exports = function (grunt) {
    grunt.initConfig({
        pkg: grunt.file.readJSON('package.json'),

        sass: {
            dist: {
                options: {
                    style: 'compressed'
                },
                files: {
                    'public/assets/css/main.css': './assets/stylesheets/main.sass'
                }
            }
        },

        postcss: {
            options: {
                map: true,
                processors: [
                    require('autoprefixer')
                ]
            },
            dist: {
                src: './public/assets/css/*.css'
            }
        },

        coffee: {
            compile: {
                files: {
                    'tmp/js/main.js': './assets/javascripts/main.coffee',
                    'tmp/js/video.js': './assets/javascripts/video.coffee',
                }
            },
        },

        uglify: {
            main: {
                src: 'tmp/js/main.js',
                dest: 'public/assets/js/main.js'
            },
            video: {
                src: 'tmp/js/video.js',
                dest: 'public/assets/js/video.js'
            }
        },

        watch: {
            css: {
                files: ['./assets/stylesheets/*.sass'],
                tasks: ['sass', 'postcss:dist'],
                options: {
                    spawn: false,
                }
            },

            js: {
                files: ['./assets/javascripts/*.coffee'],
                tasks: ['coffee', 'uglify'],
                options: {
                    spawn: false,
                }
            },
            options: {
                livereload: true,
            },
        },
    });

    grunt.loadNpmTasks('grunt-contrib-watch');

    grunt.loadNpmTasks('grunt-contrib-sass')
    grunt.loadNpmTasks('grunt-postcss');

    grunt.loadNpmTasks('grunt-contrib-coffee');
    grunt.loadNpmTasks('grunt-contrib-uglify');

    grunt.registerTask('default', ['sass', 'postcss:dist', 'coffee', 'uglify', 'watch']);

}
