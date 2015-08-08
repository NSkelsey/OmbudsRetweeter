'use strict';

angular.module("OmbudsRetweetRelay", ['markdownModule', 'ombWebAppFilters'])
.controller('topCtrl', function($scope, backendService) {
    $scope.author = {
        bltns: [],
    }
    backendService.then(function(author) {
        $scope.author = author
    });
})
.factory('backendService', function($http, $q) {

    var authorPromise = $http.get('/api/author/mvnrngzsNFdbHrRYqdZNC8Y6aoS9tZRMRu').then(function(result) {
        return result.data
    });
    return authorPromise;
})
.factory('heightService', function($http) {
    var heightP = $http.get('/api/status').then(function(result) {
        return result.data.blkCount;
    });
    return heightP;
})
.directive('pinBulletin', function() {
    return {
        templateUrl: 'pinned-bulletin.html', 
        controller: 'bltnCtrl',
        restrict: 'C',
    }
})
.controller('bltnCtrl', function($scope, markdownService, heightService) {
    // Functions to bind into the current scope:
    var bltn = $scope.bltn;
    var base = "/images/";
    $scope.depthSrc = base + "totalconf.png";

    $scope.moreDetail = function(bltn) {
        bltn.detail = !bltn.detail;
    }

    heightService.then(function(height) {
        $scope.depthSrc = depthImg(height);
    });

    var depthImg = function(height) {
        var curHeight = height;
        if (curHeight === 0) {
            return base + '0conf.png';
        }

        if (!angular.isDefined(bltn.blk)) {
            // The bltn is not mined
            return base + "0conf.png"       
        } else {
            // The bltn is in some block
            var diff = curHeight - bltn.blkHeight;

            // TODO deal with blk of unknown height
            if (diff < 0) {
                return base + "unknownconf.png"
            }

            if (diff > 3) {
                // The bltn is somewhere in the chain
                return base + "totalconf.png"
            }
            // The bltn is less than 5 blocks deep
            return base + (diff + 1) + "conf.png"
        }
    }

    $scope.renderMd = function(bltn) {
        var html = markdownService.makeHtml(bltn.msg);
        bltn.markdown = html;
    }

    bltn.renderMd = true;
    if (bltn.hasOwnProperty('msg') && bltn.msg !== "") {
        $scope.renderMd(bltn);
    }
})
.directive('authorIcon', function() {
    return {
        scope : {
            addr: '='
        },
        templateUrl: 'author-icon.html',
        restrict: 'E'
    }
});





/************* A small markdown rendering service **************/
angular.module('markdownModule', ['ngSanitize'])
.factory('markdownService', function($sanitize) {
    var conv = new Showdown.converter({}); 
    return {
        'makeHtml': function(unsafeInp) {
            var safe = $sanitize(conv.makeHtml(unsafeInp)); 
            return safe;
        }
    }
});
