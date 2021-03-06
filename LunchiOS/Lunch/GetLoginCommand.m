//
//  GetLoginCommand.m
//  Lunch
//
//  Created by Nathan Fraenkel on 10/1/15.
//  Copyright © 2015 Lunch. All rights reserved.
//

#import "GetLoginCommand.h"

@implementation GetLoginCommand

@synthesize email;

-(id)initWithEmail:(NSString*)newEmail {
    self = [super init];
    if (self) {
        self.email = newEmail;
    }
    return self;
}

-(void)login {
    NSLog(@"[GetLoginCommand] logging in.....");
    
    // spinnaz
    [UIApplication sharedApplication].networkActivityIndicatorVisible = YES;
    
    // TODO - api url
    NSString *url = [NSString stringWithFormat:@"%@/api/login", SERVER];
    
    // HTTP request, setting stuff
    NSMutableURLRequest *request = [NSMutableURLRequest requestWithURL:[NSURL URLWithString:url] cachePolicy:NSURLRequestUseProtocolCachePolicy timeoutInterval:10];
    [request setHTTPMethod:@"POST"];
    [request setValue:@"application/json" forHTTPHeaderField:@"Content-Type"];
    NSString *postString = [NSString stringWithFormat:@"{\"email\":\"%@\"}", self.email];
    [request setHTTPBody:[postString dataUsingEncoding:NSUTF8StringEncoding]];
    
    // start connection
    NSURLConnection *connection = [[NSURLConnection alloc] initWithRequest:request delegate:self];
    [connection start];
}

#pragma mark connection protocol functions
- (void)connection:(NSURLConnection *)connection didReceiveResponse:(NSURLResponse *)response {
    NSLog(@"[GetLoginCommand] conection did receive response!");
    _data = [[NSMutableData alloc] init];
}

- (void)connection:(NSURLConnection *)connection didReceiveData:(NSData *)data {
    NSLog(@"[GetLoginCommand] conection did receive data!");
    [_data appendData:data];
}

- (void)connection:(NSURLConnection *)connection didFailWithError:(NSError *)error {
    // Please do something sensible here, like log the error.
    NSLog(@"[GetLoginCommand] connection failed with error: %@", error.description);
    
    // stop spinners
    [UIApplication sharedApplication].networkActivityIndicatorVisible = NO;
    
    [self.delegate reactToLoginError:error];
}

- (void)connectionDidFinishLoading:(NSURLConnection *)connection {
    NSLog(@"[GetLoginCommand] connectiondidfinishloading!");
    [UIApplication sharedApplication].networkActivityIndicatorVisible = NO;
    
    NSDictionary *dictResponse = [NSJSONSerialization JSONObjectWithData:_data options:0 error:nil];
    
    if (dictResponse == NULL) {
        UIAlertView *alert = [[UIAlertView alloc]
                              initWithTitle:@"Bad Login Info"
                              message:@"Please try typing your email and password again."
                              delegate:self
                              cancelButtonTitle:@"OK" otherButtonTitles:nil];
        [alert show];
        return;
    }
    NSString *_id = [dictResponse objectForKey:@"id"];
    NSString *first = [dictResponse objectForKey:@"first_name"];
    NSString *last = [dictResponse objectForKey:@"last_name"];
    NSString *em = [dictResponse objectForKey:@"email"];
    NSString *photo = [dictResponse objectForKey:@"photo"];
    User *newUser = [[User alloc] initWithId:_id
                                       andFirst:first
                                        andLast:last
                                       andEmail:em
                                       andPhoto:photo];

    [self.delegate reactToLoginResponse:newUser];
    
}

@end
