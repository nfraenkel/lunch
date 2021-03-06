//
//  User.h
//  Lunch
//
//  Created by Nathan Fraenkel on 10/1/15.
//  Copyright © 2015 Lunch. All rights reserved.
//

#import <Foundation/Foundation.h>

@interface User : NSObject

@property (strong, nonatomic) NSString *_id, *first, *last, *email, *photoUrl;

-(id)initWithId:(NSString *)newId
       andFirst:(NSString *)newFirst
        andLast:(NSString *)newLast
       andEmail:(NSString *)newEmail
       andPhoto:(NSString*)newPhoto;

@end
