module ApplicationTests exposing (all)

import Application.Application as Application
import Browser
import Common exposing (queryView)
import Expect
import Message.Effects as Effects
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription as Subscription exposing (Delivery(..))
import Message.TopLevelMessage as Msgs
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Selector exposing (style)
import Url


all : Test
all =
    describe "top-level application"
        [ test "should subscribe to clicks from the not-automatically-linked boxes in the pipeline, and the token return" <|
            \_ ->
                Common.init "/teams/t/pipelines/p/"
                    |> Application.subscriptions
                    |> Expect.all
                        [ Common.contains Subscription.OnNonHrefLinkClicked
                        , Common.contains Subscription.OnTokenReceived
                        ]
        , test "clicking a not-automatically-linked box in the pipeline redirects" <|
            \_ ->
                Common.init "/teams/t/pipelines/p/"
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            NonHrefLinkClicked "/foo/bar"
                        )
                    |> Tuple.second
                    |> Expect.equal [ Effects.LoadExternal "/foo/bar" ]
        , test "received token is passed to all subsequent requests" <|
            \_ ->
                let
                    pipelineIdentifier =
                        { pipelineName = "p", teamName = "t" }
                in
                Common.init "/"
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            TokenReceived <|
                                Ok "real-token"
                        )
                    |> Tuple.first
                    |> .session
                    |> .csrfToken
                    |> Expect.equal "real-token"
        , test "subscribes to mouse events when dragging the side bar handle" <|
            \_ ->
                Common.init "/teams/t/pipelines/p/jobs/j"
                    |> Application.update
                        (Msgs.Update <|
                            Click SideBarResizeHandle
                        )
                    |> Tuple.first
                    |> Application.subscriptions
                    |> Expect.all
                        [ Common.contains Subscription.OnMouse
                        , Common.contains Subscription.OnMouseUp
                        ]
        ]
