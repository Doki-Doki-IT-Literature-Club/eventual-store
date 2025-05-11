workspace "Simple Order Processing System - Containers" "Level 2 Container diagram for the Simple Order Processing System." {

    model {
        customer = person "Customer" "An end-user of the e-commerce platform who places orders." {
            tags "Actor"
        }
        inventoryManage = person "Inventory Manager" "An internal user responsible for managing product stock levels." {
            tags "Actor"
        }
        

        inventorySystem = softwareSystem "Inventory System" "Manages product stock levels and processes inventory update events." {
            tags "ExternalSystem"

            inventoryServiceGroup = group "Inventory Service" {
                inventoryService = container "Inventory Service" "Manages product stock levels and processes inventory update events." "Node.js/Express" {
                    tags "Service"
                }

                inverntoryItemsTopic = container "Inventory Items Topic" "Topic for inventory item-related events." {
                    tags "Topic"
                }

                inventoryDb = container "Inventory Database" "Stores current stock levels for products." "Redis/PostgreSQL" {
                    tags "Database"
                }

                orderRequestTopic = container "Order Request Topic" "Topic for order requests." {
                    tags "Topic"
                }
            }
        }
        
        orderProcessingSystem = softwareSystem "Simple Order Processing System" "Handles the reception and processing of customer orders, coordinating inventory updates and user notifications asynchronously via events (EDA)." {
            orderServiceGroup = group "Order Service" {
                orderService = container "Order Service" "Handles incoming order requests, manages order state, and publishes/consumes order-related events." "Python/FastAPI" {
                    tags "Service"
                }

                orderRequestResultTopic = container "Order Request Result Topic" "Topic for order results." {
                    tags "Topic"
                }

                orderDb = container "Order Database" "Stores order information (status, items, user details)." "PostgreSQL" {
                    tags "Database"
                }

                orderStateTopic = container "Order State Topic" "Topic for compacted order state." {
                    tags "Topic"
                }
                            
                shippingStatusTopic = container "Shipping Status Topic" "Topic for shipping status updates." {
                    tags "Topic"
                }
            }

        }

        customerNotificationSystem = softwareSystem "Custromer Notification System" "Handles user notifications and alerts." {

            notificationServiceGroup = group "Custromer Notification Service" {

                notificationService = container "Custromer Notification Service" "Consumes events and sends notifications to users via external services." "Go/Gin" {
                    tags "Service"
                }
            }

        }

        inventoryNotificationSystem = softwareSystem "Inverntory Notification System" "Handles user notifications and alerts." {

            inventoryNotificationServiceGroup = group "Inverntory Notification Service" {

                inventoryNotificationService = container "Inverntory Notification Service" "Consumes events and sends notifications to users via external services." "Go/Gin" {
                    tags "Service"
                }
            }

        }

        

        identityProvider = softwareSystem "Identity Provider" "External system for authenticating users." {
            tags "ExternalSystem"
        }

        shippingSytem = softwareSystem "Shipping Service" "External system for managing shipping and delivery." {
            shippingService = container "Shipping Service" "Handles shipping-related events and updates." "Java/Spring" {
                tags "Service"
            }


            orderReadyForShippingTopic = container "Order Ready for Shipping Topic" "Topic for order ready for shipping events." {
                tags "Topic"
            }

        }

        showcaseSystem = softwareSystem "Showcase System" "External system for showcasing products." {
            tags "ExternalSystem"

            showcaseService = container "Showcase Service" "Handles product showcasing and related events." "Java/Spring" {
                tags "Service"
            }

            showcaseDb = container "Showcase Database" "Stores product information and showcases." "PostgreSQL" {
                tags "Database"
            }


        }

        orderService -> orderDb "Reads from/Writes to" "SQL"
        orderService -> orderStateTopic "Publishes"
        orderService -> orderRequestTopic "Publishes"
        orderService -> shippingStatusTopic "Consumes"
        orderService -> orderRequestResultTopic "Consumes"
        orderService -> orderReadyForShippingTopic "Publishes"

        inventoryService -> inventoryDb "Reads from/Writes to"
        inventoryService -> orderRequestTopic "Consumes"
        inventoryService -> inverntoryItemsTopic "Consumes"
        inventoryService -> inverntoryItemsTopic "Publishes" 
        inventoryService -> orderRequestResultTopic "Publishes"

        notificationService -> orderStateTopic "Consumes"

        shippingService -> orderReadyForShippingTopic "Consumes"
        shippingService -> shippingStatusTopic "Publishes"

        showcaseService -> orderStateTopic "Consumes"
        showcaseService -> inverntoryItemsTopic "Consumes"
        showcaseService -> showcaseDb "Reads from/Writes to" "JDBC"
        showcaseService -> orderService "Sends create order" "HTTP/REST"

        inventoryNotificationService -> inverntoryItemsTopic "Consumes"
    }

    views {
        systemLandscape "EventualStore" {
            include *
        }

        systemContext orderProcessingSystem "SystemContext" "The System Context diagram for the Simple Order Processing System." {
            include *
            description "Shows the Simple Order Processing System, the people who use it, and the external systems it interacts with."
        }

        container orderProcessingSystem "Containers" "The Container diagram for the Simple Order Processing System." {
            include element.parent==orderProcessingSystem element.parent==inventorySystem element.parent==shippingSytem element.parent==showcaseSystem element.parent==customerNotificationSystem element.parent==inventoryNotificationSystem
            description "Shows the internal containers within the Simple Order Processing System and their interactions."
        }

        styles {
            element "Person" { 
                shape Person
                background #d86614
                color #ffffff
                fontSize 22 
            }
            element "Software System" { 
                shape RoundedBox
                background #1168bd
                color #ffffff
                fontSize 22 
            }
            element "Container" { 
                shape Hexagon
                background #438dd5
                color #ffffff
                fontSize 20 
            }
            element "Database" { 
                shape Cylinder
                background #85bce0
                color #000000
                fontSize 18 
            }
            element "MessageBus" { 
                shape Pipe
                background #bebebe
                color #000000
                fontSize 18 
            }
            element "ExternalSystem" { 
                background #6c757d
                color #ffffff 
            }
            element "Actor" { 
                background #d86614 
            }
            element "Service" { 
                shape Component 
            }
            element "Topic" { 
                shape Pipe
                background #f0ad4e
                color #000000
                fontSize 18 
            }
        }
    }
    
}
